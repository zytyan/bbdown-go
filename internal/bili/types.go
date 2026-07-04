package bili

type Page struct {
	Index     int    `json:"index"`
	AID       int64  `json:"aid"`
	BVID      string `json:"bvid"`
	CID       int64  `json:"cid"`
	Title     string `json:"title"`
	Duration  int    `json:"duration"`
	Dimension string `json:"dimension"`
}

type VideoInfo struct {
	AID      int64  `json:"aid"`
	BVID     string `json:"bvid"`
	Title    string `json:"title"`
	Desc     string `json:"desc"`
	Pic      string `json:"pic"`
	PubTime  int64  `json:"pubtime"`
	OwnerMID int64  `json:"owner_mid"`
	Owner    string `json:"owner"`
	Pages    []Page `json:"pages"`
}

type Stream struct {
	ID        uint32   `json:"id"`
	Quality   string   `json:"quality"`
	BaseURL   string   `json:"base_url"`
	BackupURL []string `json:"backup_url,omitempty"`
	Bandwidth uint32   `json:"bandwidth"`
	Codecs    string   `json:"codecs"`
	Size      uint64   `json:"size,omitempty"`
}

type PlayInfo struct {
	DurationMS uint64   `json:"duration_ms"`
	Videos     []Stream `json:"videos"`
	Audios     []Stream `json:"audios"`
}
