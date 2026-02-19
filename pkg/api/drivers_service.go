package api

import (
	"context"
	"errors"
	"log"
	"strings"

	"connectrpc.com/connect"

	"githook/pkg/core"
	driverspkg "githook/pkg/drivers"
	cloudv1 "githook/pkg/gen/cloud/v1"
	"githook/pkg/storage"
)

// DriversService handles CRUD for driver configs.
type DriversService struct {
	Store  storage.DriverStore
	Cache  *driverspkg.Cache
	Logger *log.Logger
}

func (s *DriversService) ListDrivers(
	ctx context.Context,
	req *connect.Request[cloudv1.ListDriversRequest],
) (*connect.Response[cloudv1.ListDriversResponse], error) {
	_ = req
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	records, err := s.Store.ListDrivers(ctx)
	if err != nil {
		logError(s.Logger, "list drivers failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("list drivers failed"))
	}
	resp := &cloudv1.ListDriversResponse{
		Drivers: toProtoDriverRecords(records),
	}
	return connect.NewResponse(resp), nil
}

func (s *DriversService) GetDriver(
	ctx context.Context,
	req *connect.Request[cloudv1.GetDriverRequest],
) (*connect.Response[cloudv1.GetDriverResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	name := strings.TrimSpace(req.Msg.GetName())
	record, err := s.Store.GetDriver(ctx, name)
	if err != nil {
		logError(s.Logger, "get driver failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("get driver failed"))
	}
	if record == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("driver not found"))
	}
	resp := &cloudv1.GetDriverResponse{
		Driver: toProtoDriverRecord(record),
	}
	return connect.NewResponse(resp), nil
}

func (s *DriversService) UpsertDriver(
	ctx context.Context,
	req *connect.Request[cloudv1.UpsertDriverRequest],
) (*connect.Response[cloudv1.UpsertDriverResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	driver := req.Msg.GetDriver()
	record, err := s.Store.UpsertDriver(ctx, storage.DriverRecord{
		ID:         strings.TrimSpace(driver.GetId()),
		Name:       strings.TrimSpace(driver.GetName()),
		ConfigJSON: strings.TrimSpace(driver.GetConfigJson()),
		Enabled:    driver.GetEnabled(),
	})
	if err != nil {
		logError(s.Logger, "upsert driver failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("upsert driver failed"))
	}
	if s.Cache != nil {
		if err := s.Cache.Refresh(ctx); err != nil {
			logError(s.Logger, "driver cache refresh failed", err)
		}
	}
	resp := &cloudv1.UpsertDriverResponse{
		Driver: toProtoDriverRecord(record),
	}
	if err := ensureDriverBrokerReady(ctx, record); err != nil {
		logError(s.Logger, "driver broker validation failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("driver broker validation failed"))
	}
	return connect.NewResponse(resp), nil
}

func ensureDriverBrokerReady(ctx context.Context, record *storage.DriverRecord) error {
	if record == nil {
		return nil
	}
	cfg, err := driverspkg.ConfigFromDriver(record.Name, record.ConfigJSON)
	if err != nil {
		return err
	}
	pub, err := core.NewPublisher(cfg)
	if err != nil {
		return err
	}
	defer pub.Close()
	return nil
}

func (s *DriversService) DeleteDriver(
	ctx context.Context,
	req *connect.Request[cloudv1.DeleteDriverRequest],
) (*connect.Response[cloudv1.DeleteDriverResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	name := strings.TrimSpace(req.Msg.GetName())
	if err := s.Store.DeleteDriver(ctx, name); err != nil {
		logError(s.Logger, "delete driver failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("delete driver failed"))
	}
	if s.Cache != nil {
		if err := s.Cache.Refresh(ctx); err != nil {
			logError(s.Logger, "driver cache refresh failed", err)
		}
	}
	return connect.NewResponse(&cloudv1.DeleteDriverResponse{}), nil
}
