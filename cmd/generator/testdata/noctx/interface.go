package noctx

import "context"

type Helper interface {
	Name() string
	Sum(a, b int) (int, error)
	Run(context.Context) error
}
