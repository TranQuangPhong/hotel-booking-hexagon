package inventory

import "context"

type Repository interface {
	Create(ctx context.Context, inventories []*Inventory) error
}
