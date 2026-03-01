package staffclient

import (
	"context"

	staffv1 "github.com/RomanKovalev007/barber_crm/api/proto/staff/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn   *grpc.ClientConn
	client staffv1.StaffServiceClient
}

func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:   conn,
		client: staffv1.NewStaffServiceClient(conn),
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) GetBarber(ctx context.Context, id string) (*staffv1.BarberResponse, error) {
	return c.client.GetBarber(ctx, &staffv1.GetBarberRequest{Id: id})
}

func (c *Client) GetSchedule(ctx context.Context, barberID, week string) (*staffv1.GetScheduleResponse, error) {
	return c.client.GetSchedule(ctx, &staffv1.GetScheduleRequest{
		BarberId: barberID,
		Week:     week,
	})
}

func (c *Client) ListServices(ctx context.Context, barberID string, includeInactive bool) (*staffv1.ListServicesResponse, error) {
	return c.client.ListServices(ctx, &staffv1.ListServicesRequest{
		BarberId:        barberID,
		IncludeInactive: includeInactive,
	})
}
