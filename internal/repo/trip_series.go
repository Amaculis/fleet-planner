package repo

import (
	"context"
	"fmt"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/repo/sqlcgen"
)

func (r *Repo) CreateTripSeries(ctx context.Context, s domain.TripSeries) (domain.TripSeries, error) {
	row, err := r.q.CreateTripSeries(ctx, sqlcgen.CreateTripSeriesParams{
		Origin:        s.Origin,
		Destination:   s.Destination,
		DaysOfWeek:    s.DaysOfWeek,
		FirstStart:    s.FirstStart,
		FirstEnd:      s.FirstEnd,
		EndsOn:        s.EndsOn,
		PaymentStatus: sqlcgen.PaymentStatus(s.PaymentStatus),
		Notes:         s.Notes,
	})
	if err != nil {
		return domain.TripSeries{}, fmt.Errorf("creating trip series: %w", translate(err))
	}
	return tripSeriesFromRow(row), nil
}
