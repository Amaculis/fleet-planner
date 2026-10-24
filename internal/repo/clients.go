package repo

import (
	"context"
	"fmt"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/repo/sqlcgen"
)

func (r *Repo) ListClients(ctx context.Context) ([]domain.Client, error) {
	rows, err := r.q.ListClients(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing clients: %w", translate(err))
	}
	clients := make([]domain.Client, 0, len(rows))
	for _, row := range rows {
		clients = append(clients, clientFromRow(sqlcgen.Client(row)))
	}
	return clients, nil
}

func (r *Repo) ListActiveClients(ctx context.Context) ([]domain.Client, error) {
	rows, err := r.q.ListActiveClients(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing active clients: %w", translate(err))
	}
	clients := make([]domain.Client, 0, len(rows))
	for _, row := range rows {
		clients = append(clients, clientFromRow(sqlcgen.Client(row)))
	}
	return clients, nil
}

func (r *Repo) GetClient(ctx context.Context, id int64) (domain.Client, error) {
	row, err := r.q.GetClient(ctx, id)
	if err != nil {
		return domain.Client{}, fmt.Errorf("getting client: %w", translate(err))
	}
	return clientFromRow(sqlcgen.Client(row)), nil
}

func (r *Repo) CreateClient(ctx context.Context, c domain.Client) (domain.Client, error) {
	row, err := r.q.CreateClient(ctx, sqlcgen.CreateClientParams{
		Name:          c.Name,
		ContactPerson: c.ContactPerson,
		Phone:         c.Phone,
		Email:         c.Email,
		Address:       c.Address,
		Notes:         c.Notes,
	})
	if err != nil {
		return domain.Client{}, fmt.Errorf("creating client: %w", translate(err))
	}
	return clientFromRow(sqlcgen.Client(row)), nil
}

// UpdateClient refuses to touch an anonymised client (the query filters on
// anonymized_at IS NULL), so erased personal data cannot be written back.
func (r *Repo) UpdateClient(ctx context.Context, c domain.Client) (domain.Client, error) {
	row, err := r.q.UpdateClient(ctx, sqlcgen.UpdateClientParams{
		ID:            c.ID,
		Name:          c.Name,
		ContactPerson: c.ContactPerson,
		Phone:         c.Phone,
		Email:         c.Email,
		Address:       c.Address,
		Notes:         c.Notes,
		IsActive:      c.IsActive,
	})
	if err != nil {
		return domain.Client{}, fmt.Errorf("updating client: %w", translate(err))
	}
	return clientFromRow(sqlcgen.Client(row)), nil
}

// AnonymizeClient is the GDPR erasure path: identifying columns are cleared and the row
// is kept, so the trips that reference it stay intact.
func (r *Repo) AnonymizeClient(ctx context.Context, id int64, placeholderName string) (domain.Client, error) {
	row, err := r.q.AnonymizeClient(ctx, sqlcgen.AnonymizeClientParams{
		ID:   id,
		Name: placeholderName,
	})
	if err != nil {
		return domain.Client{}, fmt.Errorf("anonymizing client: %w", translate(err))
	}
	return clientFromRow(sqlcgen.Client(row)), nil
}
