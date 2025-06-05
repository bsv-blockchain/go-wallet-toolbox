package wdk

type EntityName string

const (
	ProvenTxEntityName       EntityName = "provenTx"
	OutputBasketEntityName  EntityName = "outputBasket"
	TransactionEntityName    EntityName = "transaction"
	ProvenTxReqEntityName    EntityName = "provenTxReq"
	TxLabelEntityName        EntityName = "txLabel"
	TxLabelMapEntityName     EntityName = "txLabelMap"
	OutputEntityName         EntityName = "output"
	OutputTagEntityName      EntityName = "outputTag"
	OutputTagMapEntityName  EntityName = "outputTagMap"
	CertificateEntityName    EntityName = "certificate"
	CertificateFieldEntityName EntityName = "certificateField"
	CommissionEntityName     EntityName = "commission"
)

var allEntityNames = []EntityName{
	ProvenTxEntityName,
	OutputBasketEntityName,
	TransactionEntityName,
	ProvenTxReqEntityName,
	TxLabelEntityName,
	TxLabelMapEntityName,
	OutputEntityName,
	OutputTagEntityName,
	OutputTagMapEntityName,
	CertificateEntityName,
	CertificateFieldEntityName,
	CommissionEntityName,
}

