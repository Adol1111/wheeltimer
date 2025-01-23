//go:build !assert

package assert

func True(val bool, msgAndArgs ...interface{}) {
}

func TrueFunc(f func() bool, msgAndArgs ...interface{}) {
}
