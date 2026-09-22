package repo

import (
	"context"
	"fmt"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/repo/sqlcgen"
)

// ---------------------------------------------------------------------------
// Buses
// ---------------------------------------------------------------------------

func (r *Repo) ListBuses(ctx context.Context) ([]domain.Bus, error) {
	rows, err := r.q.ListBuses(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing buses: %w", translate(err))
	}
	buses := make([]domain.Bus, 0, len(rows))
	for _, row := range rows {
		buses = append(buses, busFromRow(sqlcgen.Bus(row)))
	}
	return buses, nil
}

func (r *Repo) ListActiveBuses(ctx context.Context) ([]domain.Bus, error) {
	rows, err := r.q.ListActiveBuses(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing active buses: %w", translate(err))
	}
	buses := make([]domain.Bus, 0, len(rows))
	for _, row := range rows {
		buses = append(buses, busFromRow(sqlcgen.Bus(row)))
	}
	return buses, nil
}

func (r *Repo) GetBus(ctx context.Context, id int64) (domain.Bus, error) {
	row, err := r.q.GetBus(ctx, id)
	if err != nil {
		return domain.Bus{}, fmt.Errorf("getting bus: %w", translate(err))
	}
	return busFromRow(sqlcgen.Bus(row)), nil
}

func (r *Repo) CreateBus(ctx context.Context, b domain.Bus) (domain.Bus, error) {
	row, err := r.q.CreateBus(ctx, sqlcgen.CreateBusParams{
		Plate:            b.Plate,
		Model:            b.Model,
		Seats:            b.Seats,
		Status:           sqlcgen.BusStatus(b.Status),
		InsuranceExpiry:  timeToDate(b.InsuranceExpiry),
		InspectionExpiry: timeToDate(b.InspectionExpiry),
	})
	if err != nil {
		return domain.Bus{}, fmt.Errorf("creating bus: %w", translate(err))
	}
	return busFromRow(sqlcgen.Bus(row)), nil
}

func (r *Repo) UpdateBus(ctx context.Context, b domain.Bus) (domain.Bus, error) {
	row, err := r.q.UpdateBus(ctx, sqlcgen.UpdateBusParams{
		ID:               b.ID,
		Plate:            b.Plate,
		Model:            b.Model,
		Seats:            b.Seats,
		Status:           sqlcgen.BusStatus(b.Status),
		InsuranceExpiry:  timeToDate(b.InsuranceExpiry),
		InspectionExpiry: timeToDate(b.InspectionExpiry),
	})
	if err != nil {
		return domain.Bus{}, fmt.Errorf("updating bus: %w", translate(err))
	}
	return busFromRow(sqlcgen.Bus(row)), nil
}

func (r *Repo) DeleteBus(ctx context.Context, id int64) error {
	n, err := r.q.DeleteBus(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting bus: %w", translate(err))
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Drivers
// ---------------------------------------------------------------------------

func (r *Repo) ListDrivers(ctx context.Context) ([]domain.Driver, error) {
	rows, err := r.q.ListDrivers(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing drivers: %w", translate(err))
	}
	drivers := make([]domain.Driver, 0, len(rows))
	for _, row := range rows {
		drivers = append(drivers, driverFromRow(sqlcgen.Driver(row)))
	}
	return drivers, nil
}

func (r *Repo) ListActiveDrivers(ctx context.Context) ([]domain.Driver, error) {
	rows, err := r.q.ListActiveDrivers(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing active drivers: %w", translate(err))
	}
	drivers := make([]domain.Driver, 0, len(rows))
	for _, row := range rows {
		drivers = append(drivers, driverFromRow(sqlcgen.Driver(row)))
	}
	return drivers, nil
}

func (r *Repo) GetDriver(ctx context.Context, id int64) (domain.Driver, error) {
	row, err := r.q.GetDriver(ctx, id)
	if err != nil {
		return domain.Driver{}, fmt.Errorf("getting driver: %w", translate(err))
	}
	return driverFromRow(sqlcgen.Driver(row)), nil
}

func (r *Repo) CreateDriver(ctx context.Context, d domain.Driver) (domain.Driver, error) {
	rate, err := stringToNumeric(d.HourlyRate)
	if err != nil {
		return domain.Driver{}, err
	}
	row, err := r.q.CreateDriver(ctx, sqlcgen.CreateDriverParams{
		FullName:      d.FullName,
		Phone:         d.Phone,
		LicenseNumber: d.LicenseNumber,
		LicenseExpiry: timeToDate(d.LicenseExpiry),
		HourlyRate:    rate,
		PayType:       payTypeToDB(d.PayType),
	})
	if err != nil {
		return domain.Driver{}, fmt.Errorf("creating driver: %w", translate(err))
	}
	return driverFromRow(sqlcgen.Driver(row)), nil
}

// UpdateDriver refuses to touch an anonymised driver (the query filters on
// anonymized_at IS NULL), so erased personal data cannot be written back.
func (r *Repo) UpdateDriver(ctx context.Context, d domain.Driver) (domain.Driver, error) {
	rate, err := stringToNumeric(d.HourlyRate)
	if err != nil {
		return domain.Driver{}, err
	}
	row, err := r.q.UpdateDriver(ctx, sqlcgen.UpdateDriverParams{
		ID:            d.ID,
		FullName:      d.FullName,
		Phone:         d.Phone,
		LicenseNumber: d.LicenseNumber,
		LicenseExpiry: timeToDate(d.LicenseExpiry),
		HourlyRate:    rate,
		PayType:       payTypeToDB(d.PayType),
		IsActive:      d.IsActive,
	})
	if err != nil {
		return domain.Driver{}, fmt.Errorf("updating driver: %w", translate(err))
	}
	return driverFromRow(sqlcgen.Driver(row)), nil
}

// AnonymizeDriver implements the GDPR erasure path: identifying columns are cleared and
// the row is kept, so assignments, worked time and the audit trail stay intact.
func (r *Repo) AnonymizeDriver(ctx context.Context, id int64, placeholderName string) (domain.Driver, error) {
	row, err := r.q.AnonymizeDriver(ctx, sqlcgen.AnonymizeDriverParams{
		ID:       id,
		FullName: placeholderName,
	})
	if err != nil {
		return domain.Driver{}, fmt.Errorf("anonymizing driver: %w", translate(err))
	}
	return driverFromRow(sqlcgen.Driver(row)), nil
}

// GetDriverLoginUserID returns the user account tied to a driver, if one exists.
func (r *Repo) GetDriverLoginUserID(ctx context.Context, driverID int64) (int64, error) {
	id, err := r.q.GetDriverLoginUserID(ctx, &driverID)
	if err != nil {
		return 0, fmt.Errorf("getting driver login: %w", translate(err))
	}
	return id, nil
}
