package ecs

// Client は、ECS の API を呼び、デプロイのページの URL を引く
type Client struct {
	api API // ECS の API
}

// New は、Client を作る
func New(api API) *Client {
	return &Client{api: api}
}
