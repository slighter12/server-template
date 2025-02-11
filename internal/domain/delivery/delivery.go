package delivery

import (
	"context"

	"go.uber.org/fx"
)

type Delivery interface {
	Serve(lc fx.Lifecycle, ctx context.Context)
}
