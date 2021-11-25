package conductrpc

type Blocks struct {
	Round     int64
	Proposed  []byte
	Notarised []byte
}
