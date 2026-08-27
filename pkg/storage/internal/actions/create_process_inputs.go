package actions

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"strings"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seq2"
	"github.com/go-softwarelab/common/pkg/slices"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

var readyToBeInputProvenTxStatuses = []wdk.ProvenTxReqStatus{
	wdk.ProvenTxStatusUnsent,
	wdk.ProvenTxStatusUnmined,
	wdk.ProvenTxStatusUnconfirmed,
	wdk.ProvenTxStatusSending,
	wdk.ProvenTxStatusNoSend,
	wdk.ProvenTxStatusCompleted,
}

type xinputDefinition struct {
	*wdk.ValidCreateActionInput

	Satoshis      satoshi.Value
	LockingScript []byte

	knownOutput *pkgentity.Output // This is used only for known UTXOs, can be nil for unknown UTXOs
}

type xinputDefinitions []*xinputDefinition

func (inputs xinputDefinitions) iter() iter.Seq[*xinputDefinition] {
	return seq.FromSlice(inputs)
}

type processedInputsResult struct {
	Inputs          xinputDefinitions
	Beef            *transaction.Beef
	ChangeOutputIDs []uint
	KnownOutputIDs  []uint
}

type inputsProcessor struct {
	parent          *create
	ctx             context.Context
	userID          int
	reference       string
	providedInputs  []wdk.ValidCreateActionInput
	inputBEEF       []byte
	trustSelf       bool
	txIDsLookup     map[chainhash.Hash]struct{}
	beef            *transaction.Beef
	logger          *slog.Logger
	beefVerifier    wdk.BeefVerifier
	scriptsVerifier wdk.ScriptsVerifier
}

func newInputsProcessor(
	ctx context.Context,
	parent *create,
	userID int,
	reference string,
	providedInputs []wdk.ValidCreateActionInput,
	inputBEEF []byte,
	trustSelf bool,
	beefVerifier wdk.BeefVerifier,
	scriptsVerifier wdk.ScriptsVerifier,
) (*inputsProcessor, error) {
	txIDsLookup := make(map[chainhash.Hash]struct{}, len(providedInputs))
	for _, input := range providedInputs {
		txIDHash, err := chainhash.NewHashFromHex(input.Outpoint.TxID)
		if err != nil {
			return nil, fmt.Errorf("failed to parse txID %s: %w", input.Outpoint.TxID, err)
		}
		txIDsLookup[*txIDHash] = struct{}{}
	}

	logger := logging.Child(parent.logger, "inputsProcessor")
	logger = logger.With(logging.UserID(userID), logging.Reference(reference))

	return &inputsProcessor{
		ctx:             ctx,
		logger:          logger,
		parent:          parent,
		userID:          userID,
		reference:       reference,
		inputBEEF:       inputBEEF,
		trustSelf:       trustSelf,
		txIDsLookup:     txIDsLookup,
		providedInputs:  providedInputs,
		beef:            transaction.NewBeefV2(),
		beefVerifier:    beefVerifier,
		scriptsVerifier: scriptsVerifier,
	}, nil
}

func (proc *inputsProcessor) processInputs() (*processedInputsResult, error) {
	var err error

	if len(proc.providedInputs) == 0 {
		proc.logger.DebugContext(proc.ctx, "No inputs provided, skipping processing inputs")
		return &processedInputsResult{
			Beef: transaction.NewBeefV2(),
		}, nil
	}

	if len(proc.inputBEEF) > 0 {
		proc.logger.DebugContext(proc.ctx, "Processing inputBEEF")
		if err = proc.processInputBEEF(); err != nil {
			return nil, fmt.Errorf("failed to process inputBEEF: %w", err)
		}
	}

	proc.logger.DebugContext(proc.ctx, "Processing inputs")
	if err = proc.checkInputsAndMergeTxIDsToBEEF(); err != nil {
		return nil, fmt.Errorf("failed to get beef for inputs: %w", err)
	}

	// only verify beef during create action if using external input
	if proc.inputBEEF != nil {
		if err = proc.hydrateUnanchoredAncestry(); err != nil {
			return nil, fmt.Errorf("failed to complete beef ancestry from storage: %w", err)
		}

		if ok, err := proc.beefVerifier.VerifyBeef(proc.ctx, proc.beef, true); err != nil {
			return nil, fmt.Errorf("failed to verify beef: %w", err)
		} else if !ok {
			return nil, fmt.Errorf("provided beef is not valid: %s", txutils.DescribeInvalidBEEF(proc.beef))
		}
	}

	// hydrate txs in beef
	if err := txutils.HydrateBEEF(proc.beef); err != nil {
		return nil, fmt.Errorf("failed to hydrate beef for script verification: %w", err)
	}

	// verify scripts for all unmined transactions in BEEF
	for txIDHash, beefTx := range proc.beef.Transactions {
		// no raw tx available or skip already mined txs
		if beefTx.Transaction == nil || beefTx.Transaction.MerklePath != nil {
			continue
		}

		if ok, err := proc.scriptsVerifier.VerifyScripts(proc.ctx, beefTx.Transaction); err != nil {
			return nil, fmt.Errorf("script verification failed for tx %s : %w", txIDHash, err)
		} else if !ok {
			return nil, fmt.Errorf("scripts are not valid for tx %s", txIDHash)
		}
	}

	return proc.buildInputsDefinition()
}

func (proc *inputsProcessor) buildInputsDefinition() (*processedInputsResult, error) {
	xinputDefs := make([]*xinputDefinition, 0, len(proc.providedInputs))
	var changeOutputIDs []uint
	var knownOutputIDs []uint
	for _, xinput := range proc.providedInputs {
		output, err := proc.parent.outputRepo.FindOutput(proc.ctx, proc.userID, xinput.Outpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to find output for input %s: %w", xinput.Outpoint, err)
		}

		var newXInput *xinputDefinition
		if output != nil {
			if output.Change {
				changeOutputIDs = append(changeOutputIDs, output.ID)
			} else {
				knownOutputIDs = append(knownOutputIDs, output.ID)
			}
			newXInput, err = proc.xinputDefOnKnownUTXO(&xinput, output)
		} else {
			newXInput, err = proc.xinputDefOnUnknownUTXO(&xinput)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to process input %s: %w", xinput.Outpoint, err)
		}

		xinputDefs = append(xinputDefs, newXInput)
	}

	return &processedInputsResult{
		Inputs:          xinputDefs,
		Beef:            proc.beef,
		ChangeOutputIDs: changeOutputIDs,
		KnownOutputIDs:  knownOutputIDs,
	}, nil
}

func (proc *inputsProcessor) processInputBEEF() error {
	var err error

	if err = proc.beef.MergeBeefBytes(proc.inputBEEF); err != nil {
		return fmt.Errorf("failed to merge input beef: %w", err)
	}

	txIDOnlyIDs := seq2.Keys(seq2.Filter(maps.All(proc.beef.Transactions), func(_ chainhash.Hash, beefTx *transaction.BeefTx) bool {
		return beefTx.DataFormat == transaction.TxIDOnly
	}))

	if !proc.trustSelf && seq.IsNotEmpty(txIDOnlyIDs) {
		return missingProofError(toStringIDs(seq.Collect(txIDOnlyIDs)), "inputBEEF contains transactions with TxIDOnly that causes error if trustSelf not set")
	}

	// not provided in inputs but exists in the inputBEEF
	notProvidedInInputs := seq.Filter(txIDOnlyIDs, func(txIDHash chainhash.Hash) bool {
		_, ok := proc.txIDsLookup[txIDHash]
		return !ok
	})

	notProvidedInInputsTxIDs := seq.Collect(seq.Map(notProvidedInInputs, func(txIDHash chainhash.Hash) string {
		return txIDHash.String()
	}))

	if len(notProvidedInInputsTxIDs) > 0 {
		allKnown, err := proc.parent.knownTxRepo.AllKnownTxsExist(proc.ctx, notProvidedInInputsTxIDs, readyToBeInputProvenTxStatuses)
		if err != nil {
			return fmt.Errorf("failed to check if transactions are known: %w", err)
		}

		if !allKnown {
			return missingProofError(notProvidedInInputsTxIDs, "some tx in the inputBEEF is not known to storage")
		}
	}

	// Runs only once the BEEF has been accepted, so a rejected request leaves no
	// rows behind.
	return proc.registerBeefAncestors()
}

// registerBeefAncestors records every transaction the submitted BEEF actually
// carries as a known tx row.
//
// This is what lets the blob stop being stored: the ancestry it holds is the
// only copy of those transactions, and duplicating it into each descendant is
// what makes storage grow with the square of a chain's length. Written as rows,
// each ancestor is stored once and the ancestry walk reads it directly.
//
// TxIDOnly entries carry no transaction, so there is nothing to record - they
// were already validated against storage above.
func (proc *inputsProcessor) registerBeefAncestors() error {
	ancestors := make([]entity.AncestorTx, 0, len(proc.beef.Transactions))

	for txIDHash, beefTx := range proc.beef.Transactions {
		if beefTx.DataFormat == transaction.TxIDOnly || beefTx.Transaction == nil {
			continue
		}

		ancestor := entity.AncestorTx{
			TxID:  txIDHash.String(),
			RawTx: beefTx.Transaction.Bytes(),
		}

		if beefTx.DataFormat == transaction.RawTxAndBumpIndex &&
			beefTx.BumpIndex >= 0 && beefTx.BumpIndex < len(proc.beef.BUMPs) {
			ancestor.MerklePath = proc.beef.BUMPs[beefTx.BumpIndex].Bytes()
		}

		ancestors = append(ancestors, ancestor)
	}

	if err := proc.parent.knownTxRepo.RegisterAncestors(proc.ctx, proc.reference, ancestors); err != nil {
		return fmt.Errorf("failed to register inputBEEF ancestors: %w", err)
	}

	proc.logger.DebugContext(proc.ctx, "Registered inputBEEF ancestors as known txs", slog.Int("count", len(ancestors)))
	return nil
}

func (proc *inputsProcessor) checkInputsAndMergeTxIDsToBEEF() error {
	missingFullProofs := seq.Collect(seq.Filter(maps.Keys(proc.txIDsLookup), func(txID chainhash.Hash) bool {
		btx, ok := proc.beef.Transactions[txID]
		return !ok || btx.DataFormat == transaction.TxIDOnly
	}))

	missingFullProofsTxIDs := slices.Map(missingFullProofs, func(txID chainhash.Hash) string {
		return txID.String()
	})

	if len(missingFullProofsTxIDs) == 0 {
		return nil
	}

	if !proc.trustSelf {
		return missingProofError(missingFullProofsTxIDs, "provided inputs contain transactions that are missing full proof in the inputBEEF")
	}

	allKnown, err := proc.parent.knownTxRepo.AllKnownTxsExist(proc.ctx, missingFullProofsTxIDs, readyToBeInputProvenTxStatuses)
	if err != nil {
		return fmt.Errorf("failed to check if transactions are known: %w", err)
	}

	if !allKnown {
		return missingProofError(missingFullProofsTxIDs, "some tx used in provided input is not known to storage")
	}

	for _, txIDHash := range missingFullProofs {
		proc.beef.MergeTxidOnly(&txIDHash)
	}

	return nil
}

// maxAncestryHydrationRounds bounds the hydrate-and-recheck loop in
// hydrateUnanchoredAncestry. One round is enough for a BEEF whose stubs sit
// directly above raw transactions; further rounds only matter when the stored
// ancestry pulled in from storage itself carries a stub with a raw child (rows
// written by older releases). The bound exists so a pathological graph cannot
// spin here.
const maxAncestryHydrationRounds = 8

// hydrateUnanchoredAncestry fills in the ancestry the BEEF validator needs but
// the provided inputBEEF does not carry.
//
// Under trustSelf a caller may replace any transaction storage already knows
// with a bare txid (the knownTxIds optimisation). That is sound for a
// transaction nothing else in the BEEF spends, but a TxIDOnly entry is not an
// anchor: go-sdk's ValidateTransactions can only call an unproven raw
// transaction valid by tracing every one of its inputs to a merkle proof. A raw
// transaction sitting above a stub - or above an ancestor the caller trimmed
// away entirely - therefore makes the whole BEEF invalid, and createAction
// rejects it with a bare "provided beef is not valid" for transactions storage
// is holding itself. That is the failure: the check just above this one proves
// storage knows them.
//
// So every source that an unproven raw transaction spends and the BEEF cannot
// anchor is read back out of storage with its full ancestry. Stubs that nothing
// in the BEEF spends stay stubs, which is what keeps the optimisation worth
// having: the common case (a caller listing its inputs as bare txids and
// sending no raw ancestry at all) still costs no ancestry walk.
func (proc *inputsProcessor) hydrateUnanchoredAncestry() error {
	if !proc.trustSelf {
		// Without trustSelf the caller promised a complete BEEF. Keep that
		// strictness and let verification reject an incomplete one.
		return nil
	}

	for round := 0; round < maxAncestryHydrationRounds; round++ {
		needed := proc.unanchorableSources()
		if len(needed) == 0 {
			return nil
		}

		allKnown, err := proc.parent.knownTxRepo.AllKnownTxsExist(proc.ctx, needed, readyToBeInputProvenTxStatuses)
		if err != nil {
			return fmt.Errorf("failed to check if ancestor transactions are known: %w", err)
		}

		if !allKnown {
			return missingProofError(needed, "inputBEEF spends transactions whose proof is neither provided nor known to storage")
		}

		proc.logger.DebugContext(proc.ctx, "Completing inputBEEF ancestry from storage",
			slog.Int("round", round),
			slog.Int("txIDsToHydrate", len(needed)),
		)

		if _, err = proc.parent.knownTxRepo.GetBEEFForTxIDs(
			proc.ctx,
			seq.FromSlice(needed),
			entity.WithMergeToBEEF(proc.beef),
			entity.WithStatusesToFilterOut(wdk.ProvenTxReqProblematicStatuses...),
		); err != nil {
			return fmt.Errorf("failed to merge storage ancestry for %s: %w", strings.Join(needed, ", "), err)
		}
	}

	return fmt.Errorf("inputBEEF ancestry still incomplete after %d hydration rounds", maxAncestryHydrationRounds)
}

// unanchorableSources lists the sources spent by unproven raw transactions in
// the BEEF that the BEEF does not carry as raw data: entries held as a bare
// txid, and sources absent altogether. A transaction that carries a proof is
// terminal - the validator never inspects its inputs and script verification
// skips it - so neither it nor anything above it is reported.
//
// A bare txid covered by a BUMP is a valid anchor for the validator but still
// has no output scripts, which the script verification that follows needs for
// every unproven raw transaction. It is reported too.
func (proc *inputsProcessor) unanchorableSources() []string {
	proven := provenInBEEF(proc.beef)

	var needed []string
	seen := make(map[chainhash.Hash]struct{})
	for txIDHash, beefTx := range proc.beef.Transactions {
		if beefTx.Transaction == nil {
			// A bare txid spends nothing as far as this BEEF is concerned.
			continue
		}
		if _, ok := proven[txIDHash]; ok {
			continue
		}

		for _, input := range beefTx.Transaction.Inputs {
			if input.SourceTXID == nil {
				continue
			}
			if _, dup := seen[*input.SourceTXID]; dup {
				continue
			}
			if source, inBEEF := proc.beef.Transactions[*input.SourceTXID]; inBEEF && source.Transaction != nil {
				continue
			}

			seen[*input.SourceTXID] = struct{}{}
			needed = append(needed, input.SourceTXID.String())
		}
	}

	return needed
}

// provenInBEEF returns the txids the BEEF can anchor on its own: every txid leaf
// carried by one of its BUMPs, plus every transaction that arrived with a merkle
// path attached. It mirrors what go-sdk's ValidateTransactions treats as
// terminal, so the two agree on which transactions still need their ancestry.
func provenInBEEF(beef *transaction.Beef) map[chainhash.Hash]struct{} {
	proven := make(map[chainhash.Hash]struct{})

	for _, bump := range beef.BUMPs {
		if len(bump.Path) == 0 {
			continue
		}
		for _, leaf := range bump.Path[0] {
			if leaf.Hash != nil && leaf.Txid != nil && *leaf.Txid {
				proven[*leaf.Hash] = struct{}{}
			}
		}
	}

	for txIDHash, beefTx := range beef.Transactions {
		if beefTx.Transaction != nil && beefTx.Transaction.MerklePath != nil {
			proven[txIDHash] = struct{}{}
		}
	}

	return proven
}

func (proc *inputsProcessor) xinputDefOnKnownUTXO(xinput *wdk.ValidCreateActionInput, output *pkgentity.Output) (*xinputDefinition, error) {
	if len(output.LockingScript) == 0 || output.Satoshis <= 0 {
		return nil, fmt.Errorf("output %s has no locking script or positive satoshis", xinput.Outpoint)
	}

	if !output.Spendable {
		return nil, fmt.Errorf("output %s is not spendable", xinput.Outpoint)
	}

	return &xinputDefinition{
		ValidCreateActionInput: xinput,
		Satoshis:               satoshi.MustFrom(output.Satoshis),
		LockingScript:          output.LockingScript,
		knownOutput:            output,
	}, nil
}

func (proc *inputsProcessor) xinputDefOnUnknownUTXO(xinput *wdk.ValidCreateActionInput) (*xinputDefinition, error) {
	txIDHash, err := chainhash.NewHashFromHex(xinput.Outpoint.TxID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse txID %s: %w", xinput.Outpoint.TxID, err)
	}

	btx, ok := proc.beef.Transactions[*txIDHash]
	if !ok || btx == nil {
		return nil, fmt.Errorf("input %s not found in beef or outputs", xinput.Outpoint)
	}

	if btx.DataFormat == transaction.TxIDOnly {
		// Filter out the problematic statuses, not the usable ones: this lookup
		// exists to read back a transaction the checks above already proved
		// storage holds at a readyToBeInputProvenTxStatuses status, so excluding
		// exactly those statuses made every stubbed unknown UTXO fail with
		// "is not known to storage" right after storage confirmed it knows it.
		beefForTx, err := proc.parent.knownTxRepo.GetBEEFForTxID(proc.ctx, xinput.Outpoint.TxID, entity.WithStatusesToFilterOut(wdk.ProvenTxReqProblematicStatuses...))
		if err != nil {
			return nil, fmt.Errorf("failed to build beef for tx %s: %w", xinput.Outpoint.TxID, err)
		}

		btx, ok = beefForTx.Transactions[*txIDHash]
		if !ok || btx == nil {
			return nil, fmt.Errorf("tx %s not found in beef", xinput.Outpoint.TxID)
		}

		if _, err = proc.beef.MergeBeefTx(btx); err != nil {
			return nil, fmt.Errorf("failed to merge beef for tx %s: %w", xinput.Outpoint.TxID, err)
		}
	}

	voutInt := must.ConvertToIntFromUnsigned(xinput.Outpoint.Vout)
	if voutInt >= len(btx.Transaction.Outputs) {
		return nil, fmt.Errorf("input %s has invalid vout %d for tx %s with outputs count %d",
			xinput.Outpoint, xinput.Outpoint.Vout, xinput.Outpoint.TxID, len(btx.Transaction.Outputs))
	}

	out := btx.Transaction.Outputs[voutInt]

	return &xinputDefinition{
		ValidCreateActionInput: xinput,
		Satoshis:               satoshi.MustFrom(out.Satoshis),
		LockingScript:          out.LockingScript.Bytes(),
	}, nil
}

func missingProofError(txIDs []string, msgParts ...string) error {
	if len(txIDs) == 0 {
		return fmt.Errorf("%s", strings.Join(msgParts, "; "))
	}

	var subject string
	if len(txIDs) > 1 {
		subject = "transactions"
	} else {
		subject = "transaction"
	}

	// The txids are the whole point of this error: without them a caller cannot
	// tell which transaction to resend. This joined msgParts, repeating the
	// message where the list belongs.
	txMsgPart := fmt.Sprintf("valid and contain complete proof data for %s: %s", subject, strings.Join(txIDs, ", "))
	if len(msgParts) > 0 {
		return fmt.Errorf("%s; %s", strings.Join(msgParts, "; "), txMsgPart)
	}
	return fmt.Errorf("%s", txMsgPart)
}

func toStringIDs(hashes []chainhash.Hash) []string {
	return slices.Map(hashes, func(hash chainhash.Hash) string {
		return hash.String()
	})
}
