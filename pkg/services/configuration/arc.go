package configuration

// ARC is a configuration for ARCService used by the wallet service to communicate with ARC.
type ARC struct {
	// URL is the base URL of the ARC service.
	URL string `mapstructure:"url"`
	// Token is the authentication token for the ARC service.
	Token string `mapstructure:"token"`
	// DeploymentID is the ID of this deployment to be announced to ARC - this is helpful for issue tracking.
	DeploymentID string `mapstructure:"deployment_id"`
	// WaitFor is the transaction status for which ARCService should wait when broadcasting transaction.
	WaitFor string `mapstructure:"wait_for"`
	// CallbackURL is the URL to which ARC will send a callback after processing the transaction.
	CallbackURL string `mapstructure:"callback_url"`
	// CallbackToken is the token used for authentication in the callback URL.
	CallbackToken string `mapstructure:"callback_token"`
}
