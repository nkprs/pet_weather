package rpc

import (
	weatherpb "weather/proto"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	Weather weatherpb.WeatherServiceClient
	conn    *grpc.ClientConn
}

func NewClient(addr string) (*Client, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		Weather: weatherpb.NewWeatherServiceClient(conn),
		conn:    conn,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
