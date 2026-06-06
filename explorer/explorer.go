package explorer

type ChainStats struct {
	Height            uint64 `json:"height"`
	Difficulty        uint32 `json:"difficulty"`
	MempoolCount      int    `json:"mempoolCount"`
	CirculatingSupply uint64 `json:"circulatingSupply"`
}
