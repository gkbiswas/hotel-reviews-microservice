package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
)

// HotelServiceAdapter adapts ReviewService to implement HotelService
type HotelServiceAdapter struct {
	reviewService domain.ReviewService
}

func NewHotelServiceAdapter(reviewService domain.ReviewService) domain.HotelService {
	return &HotelServiceAdapter{reviewService: reviewService}
}

func (h *HotelServiceAdapter) CreateHotel(ctx context.Context, hotel *domain.Hotel) error {
	return h.reviewService.CreateHotel(ctx, hotel)
}

func (h *HotelServiceAdapter) GetHotel(ctx context.Context, id uuid.UUID) (*domain.Hotel, error) {
	return h.reviewService.GetHotelByID(ctx, id)
}

func (h *HotelServiceAdapter) GetHotels(ctx context.Context, filter domain.HotelFilter) ([]*domain.Hotel, error) {
	// Convert slice to pointer slice for interface compatibility
	hotels, err := h.reviewService.ListHotels(ctx, filter.Limit, filter.Offset)
	if err != nil {
		return nil, err
	}
	
	pointerHotels := make([]*domain.Hotel, len(hotels))
	for i := range hotels {
		pointerHotels[i] = &hotels[i]
	}
	return pointerHotels, nil
}

func (h *HotelServiceAdapter) UpdateHotel(ctx context.Context, hotel *domain.Hotel) error {
	return h.reviewService.UpdateHotel(ctx, hotel)
}

func (h *HotelServiceAdapter) DeleteHotel(ctx context.Context, id uuid.UUID) error {
	return h.reviewService.DeleteHotel(ctx, id)
}

func (h *HotelServiceAdapter) GetHotelsByCity(ctx context.Context, city string, limit, offset int) ([]*domain.Hotel, error) {
	// This would need filtering support in ReviewService, for now return empty
	return []*domain.Hotel{}, nil
}

func (h *HotelServiceAdapter) GetHotelsByRating(ctx context.Context, minRating int, limit, offset int) ([]*domain.Hotel, error) {
	// This would need filtering support in ReviewService, for now return empty  
	return []*domain.Hotel{}, nil
}

// ProviderServiceAdapter adapts ReviewService to implement ProviderService
type ProviderServiceAdapter struct {
	reviewService domain.ReviewService
}

func NewProviderServiceAdapter(reviewService domain.ReviewService) domain.ProviderService {
	return &ProviderServiceAdapter{reviewService: reviewService}
}

func (p *ProviderServiceAdapter) CreateProvider(ctx context.Context, provider *domain.Provider) error {
	return p.reviewService.CreateProvider(ctx, provider)
}

func (p *ProviderServiceAdapter) GetProvider(ctx context.Context, id uuid.UUID) (*domain.Provider, error) {
	return p.reviewService.GetProviderByID(ctx, id)
}

func (p *ProviderServiceAdapter) GetProviders(ctx context.Context, filter domain.ProviderFilter) ([]*domain.Provider, error) {
	// Convert slice to pointer slice for interface compatibility
	providers, err := p.reviewService.ListProviders(ctx, filter.Limit, filter.Offset)
	if err != nil {
		return nil, err
	}
	
	pointerProviders := make([]*domain.Provider, len(providers))
	for i := range providers {
		pointerProviders[i] = &providers[i]
	}
	return pointerProviders, nil
}

func (p *ProviderServiceAdapter) UpdateProvider(ctx context.Context, provider *domain.Provider) error {
	return p.reviewService.UpdateProvider(ctx, provider)
}

func (p *ProviderServiceAdapter) DeleteProvider(ctx context.Context, id uuid.UUID) error {
	return p.reviewService.DeleteProvider(ctx, id)
}

func (p *ProviderServiceAdapter) GetActiveProviders(ctx context.Context) ([]*domain.Provider, error) {
	// This would need filtering support in ReviewService, for now return all
	providers, err := p.reviewService.ListProviders(ctx, 100, 0)
	if err != nil {
		return nil, err
	}
	
	pointerProviders := make([]*domain.Provider, len(providers))
	for i := range providers {
		pointerProviders[i] = &providers[i]
	}
	return pointerProviders, nil
}

func (p *ProviderServiceAdapter) ToggleProviderStatus(ctx context.Context, id uuid.UUID) (*domain.Provider, error) {
	// This would need implementation in ReviewService, for now just get the provider
	return p.reviewService.GetProviderByID(ctx, id)
}

func (p *ProviderServiceAdapter) ProcessReviewBatch(ctx context.Context, reviews []domain.Review) error {
	// This would need batch processing support in ReviewService
	return nil
}

func (p *ProviderServiceAdapter) ImportReviewsFromFile(ctx context.Context, fileURL string, providerID uuid.UUID) error {
	// This would use the file processing functionality
	_, err := p.reviewService.ProcessReviewFile(ctx, fileURL, providerID)
	return err
}

func (p *ProviderServiceAdapter) ExportReviewsToFile(ctx context.Context, filters map[string]interface{}, format string) (string, error) {
	// This would need export functionality, for now return empty
	return "", nil
}