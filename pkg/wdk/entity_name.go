package wdk

// EntityName represents the name of an entity aligned with the API (NOTE: These are not the same as the database table names in this project).
type EntityName string

// All entity names used in the storage API
const (
	ProvenTxEntityName         EntityName = "provenTx"
	OutputBasketEntityName     EntityName = "outputBasket"
	TransactionEntityName      EntityName = "transaction"
	ProvenTxReqEntityName      EntityName = "provenTxReq"
	TxLabelEntityName          EntityName = "txLabel"
	TxLabelMapEntityName       EntityName = "txLabelMap"
	OutputEntityName           EntityName = "output"
	OutputTagEntityName        EntityName = "outputTag"
	OutputTagMapEntityName     EntityName = "outputTagMap"
	CertificateEntityName      EntityName = "certificate"
	CertificateFieldEntityName EntityName = "certificateField"
	CommissionEntityName       EntityName = "commission"
)

// AllEntityNames contains the ordered list of all entity names used for synchronization in the application.
var AllEntityNames = []EntityName{
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
