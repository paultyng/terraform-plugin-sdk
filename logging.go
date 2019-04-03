package sdk

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
)

func LoggingServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		var (
			err  error
			resp interface{}
		)

		start := time.Now()
		defer func() {
			elapsed := time.Since(start)
			elapsed = elapsed.Round(time.Millisecond)
			method := info.FullMethod
			err := err
			log.Printf("[INFO] GRPC: method=%s elapsed=%vs", method, elapsed.Seconds())
			if err != nil {
				log.Printf("[ERROR] GRPC: method=%s elapsed=%vs err=%s", method, elapsed.Seconds(), err)
				log.Printf("[ERROR] GRPC: %+v", err)
			}
		}()

		resp, err = handler(ctx, req)
		return resp, err
	}
}
