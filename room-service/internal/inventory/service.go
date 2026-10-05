package inventory

type Service struct {
	repository Repository
}

func NewService() *Service {
	return &Service{}
}

// func (s *Service) Create(ctx context.Context, inventories []*Inventory) error {
// 	err := s.repository.Create(ctx, inventories)
// 	if err != nil {
// 		return fmt.Errorf("failed to create inventory: %w", err)
// 	}

// 	return nil
// }
