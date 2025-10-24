package chaintracks

import "context"

// Storage defines an interface for storage backends capable of storing chaintracks data.
type Storage interface{
	Migrate(ctx context.Context) error
}
