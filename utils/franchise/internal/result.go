package internal

type Result[T any] struct {
	Status string `json:"status"`
	Code   int    `json:"code"`
	Data   T      `json:"result"`
}

type Data[T any] struct {
	Status string `json:"status"`
	Code   int    `json:"code"`
	Data   T      `json:"data"`
}
