package pos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

type Cashier struct {
	Identity string `json:"identity"`
	Name     string `json:"name"`
	State    string `json:"state"`
	Invite   string `json:"invite"`
}
type Sale struct {
	Request *Signed `json:"request"`
	Frame   *Frame  `json:"frame,omitempty"`
	Receipt *Signed `json:"receipt,omitempty"`
}
type State struct {
	Invites  map[string]*Signed  `json:"invites"`
	Cashiers map[string]*Cashier `json:"cashiers"`
	Sales    map[string]*Sale    `json:"sales"`
	Nonces   map[string]int64    `json:"nonces"`
}

func EmptyState() *State {
	return &State{map[string]*Signed{}, map[string]*Cashier{}, map[string]*Sale{}, map[string]int64{}}
}

type Store interface {
	Load(context.Context) (*State, error)
	Save(context.Context, *State) error
}
type Config struct {
	Wallet   Signer
	Merchant string
	Name     string
	URL      string
	Chain    string
	Store    Store
	// Accept validates the exact amount and BRC-29 script, validates BEEF,
	// and internalizes with the merchant signing wallet. Use MerchantAcceptor.
	Accept func(context.Context, Frame, int64) (string, error)
}
type Service struct {
	config Config
	mu     sync.Mutex
}

func New(config Config) (*Service, error) {
	if _, err := identity(config.Merchant); err != nil {
		return nil, err
	}
	if err := validateURL(config.URL); err != nil {
		return nil, err
	}
	if config.Wallet == nil || config.Store == nil || config.Accept == nil || strings.TrimSpace(config.Name) == "" || len(config.Name) > 80 {
		return nil, errors.New("incomplete merchant service configuration")
	}
	if config.Chain != "main" && config.Chain != "test" && config.Chain != "teratest" {
		return nil, errors.New("invalid chain")
	}
	public, err := config.Wallet.GetPublicKey(context.Background(), sdk.GetPublicKeyArgs{IdentityKey: true}, originator)
	if err != nil {
		return nil, err
	}
	if public == nil || public.PublicKey == nil || !strings.EqualFold(public.PublicKey.ToDERHex(), config.Merchant) {
		return nil, errors.New("merchant signer identity does not match")
	}
	config.Merchant = strings.ToLower(config.Merchant)
	return &Service{config: config}, nil
}

func (s *Service) Call(ctx context.Context, envelope Signed) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := Verify(ctx, envelope); err != nil {
		return nil, err
	}
	var p RPC
	if err := json.Unmarshal(envelope.Payload, &p); err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	if p.Version != 1 || len(p.Nonce) != 32 || !isHex(p.Nonce) || p.Time < now-60000 || p.Time > now+60000 || len(p.Body) == 0 {
		return nil, errors.New("expired or invalid request")
	}
	var body struct {
		Invite  string `json:"invite"`
		Name    string `json:"name"`
		Cashier string `json:"cashier"`
		Session string `json:"session"`
		Invoice string `json:"invoice"`
		Frame   *Frame `json:"frame"`
	}
	if err := json.Unmarshal(p.Body, &body); err != nil {
		return nil, err
	}
	state, err := s.config.Store.Load(ctx)
	if err != nil {
		return nil, err
	}
	caller := strings.ToLower(envelope.IdentityKey)
	owner := caller == s.config.Merchant
	nonce := caller + ":" + p.Nonce
	if _, used := state.Nonces[nonce]; used {
		return nil, errors.New("request replayed")
	}
	for n, t := range state.Nonces {
		if t < now-120000 {
			delete(state.Nonces, n)
		}
	}
	state.Nonces[nonce] = now
	if err = s.config.Store.Save(ctx, state); err != nil {
		return nil, err
	}
	active := state.Cashiers[caller] != nil && state.Cashiers[caller].State == "active"
	save := func(result any) (any, error) { return result, s.config.Store.Save(ctx, state) }
	switch p.Method {
	case "info":
		return map[string]any{"merchant": s.config.Merchant, "name": s.config.Name, "chain": s.config.Chain, "cashier": state.Cashiers[caller]}, nil
	case "invite":
		if !owner {
			return nil, errors.New("merchant authorization required")
		}
		id := randomID()
		invite, signErr := Sign(ctx, s.config.Wallet, Invite{"invite", s.config.Merchant, s.config.Name, strings.TrimRight(s.config.URL, "/"), s.config.Chain, id, now + 600000})
		if signErr != nil {
			return nil, signErr
		}
		state.Invites[id] = invite
		return save(invite)
	case "enroll":
		invite := state.Invites[body.Invite]
		if invite == nil {
			return nil, errors.New("invitation not found")
		}
		var terms Invite
		if err = json.Unmarshal(invite.Payload, &terms); err != nil {
			return nil, err
		}
		if terms.Expires <= now {
			return nil, errors.New("invitation expired")
		}
		if state.Cashiers[caller] != nil && state.Cashiers[caller].State == "active" {
			return state.Cashiers[caller], nil
		}
		name := strings.TrimSpace(body.Name)
		if name == "" || len(name) > 80 {
			return nil, errors.New("invalid cashier name")
		}
		c := &Cashier{caller, name, "pending", body.Invite}
		state.Cashiers[caller] = c
		return save(c)
	case "cashiers":
		if !owner {
			return nil, errors.New("merchant authorization required")
		}
		result := make([]*Cashier, 0, len(state.Cashiers))
		for _, c := range state.Cashiers {
			result = append(result, c)
		}
		return result, nil
	case "approve", "revoke":
		if !owner {
			return nil, errors.New("merchant authorization required")
		}
		c := state.Cashiers[strings.ToLower(body.Cashier)]
		if c == nil {
			return nil, errors.New("cashier not found")
		}
		if p.Method == "approve" {
			inv := state.Invites[c.Invite]
			if inv == nil {
				return nil, errors.New("invitation consumed")
			}
			var terms Invite
			if err = json.Unmarshal(inv.Payload, &terms); err != nil {
				return nil, err
			}
			if terms.Expires <= now {
				return nil, errors.New("invitation expired")
			}
			delete(state.Invites, c.Invite)
			c.State = "active"
		} else {
			c.State = "revoked"
		}
		return save(c)
	case "sales":
		if !owner && !active {
			return nil, errors.New("cashier access inactive")
		}
		rows := make([]map[string]any, 0)
		for _, sale := range state.Sales {
			var r Request
			if err = json.Unmarshal(sale.Request.Payload, &r); err != nil {
				return nil, err
			}
			if owner || r.Cashier == caller {
				rows = append(rows, map[string]any{"request": sale.Request, "receipt": sale.Receipt, "pending": sale.Frame != nil && sale.Receipt == nil})
			}
		}
		return rows, nil
	case "invoice":
		if !active {
			return nil, errors.New("cashier access inactive")
		}
		session, sessionErr := decodeSession(body.Session)
		if sessionErr != nil {
			return nil, sessionErr
		}
		if !strings.EqualFold(session.Identity, s.config.Merchant) {
			return nil, errors.New("incorrect payee")
		}
		id := randomID()
		request, requestErr := Sign(ctx, s.config.Wallet, Request{"payment", s.config.Merchant, s.config.Name, caller, id, s.config.Chain, now + 300000, body.Session})
		if requestErr != nil {
			return nil, requestErr
		}
		state.Sales[id] = &Sale{Request: request}
		return save(request)
	case "status", "settle":
		if !active {
			return nil, errors.New("cashier access inactive")
		}
		sale := state.Sales[body.Invoice]
		if sale == nil {
			return nil, errors.New("invoice not found")
		}
		var terms Request
		if err = json.Unmarshal(sale.Request.Payload, &terms); err != nil {
			return nil, err
		}
		if terms.Cashier != caller {
			return nil, errors.New("invoice belongs to another cashier")
		}
		if p.Method == "status" {
			return map[string]any{"receipt": sale.Receipt, "pending": sale.Frame != nil}, nil
		}
		f := body.Frame
		session, err := decodeSession(terms.Session)
		if err != nil {
			return nil, err
		}
		if f == nil || f.Version != 5 || f.Kind != "bsv" || len(f.Token) > 0 || f.DerivationPrefix != session.Prefix || f.DerivationSuffix != session.Suffix {
			return nil, errors.New("payment does not match invoice")
		}
		if _, err = identity(f.SenderIdentityKey); err != nil {
			return nil, err
		}
		if _, err = byteArray(f.Transaction, 2000000); err != nil {
			return nil, err
		}
		if sale.Frame != nil {
			old, _ := marshal(sale.Frame)
			next, _ := marshal(f)
			if !bytes.Equal(old, next) {
				return nil, errors.New("different payment already submitted")
			}
		}
		if sale.Receipt != nil {
			return sale.Receipt, nil
		}
		if sale.Frame == nil && terms.Expires <= now {
			return nil, errors.New("invoice expired")
		}
		sale.Frame = f
		if err = s.config.Store.Save(ctx, state); err != nil {
			return nil, err
		}
		txid, err := s.config.Accept(ctx, *f, session.Amount)
		if err != nil {
			return nil, err
		}
		if len(txid) != 64 || !isHex(txid) {
			return nil, errors.New("invalid wallet transaction identity")
		}
		receipt, err := Sign(ctx, s.config.Wallet, Receipt{terms.Invoice, txid, session.Amount, "accepted"})
		if err != nil {
			return nil, err
		}
		sale.Receipt = receipt
		return save(receipt)
	}
	return nil, errors.New("unsupported request")
}

func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
