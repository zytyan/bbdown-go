package bili

import (
	"errors"
	"strconv"
	"strings"
)

const (
	bvTable = "FcwAPNKTMug3GV5Lj7EJnHpWsx4tb8haYeviqBz6rkCy12mUSDQX9RdoZf"
	bvXor   = 23442827791579
	bvMask  = (int64(1) << 51) - 1
	bvMax   = bvMask + 1
)

type InputID struct {
	AID      int64
	BVID     string
	FileName string
}

func ParseInputID(input string) (InputID, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return InputID{}, errors.New("empty url or id")
	}
	if i := strings.Index(s, "BV"); i >= 0 && len(s[i:]) >= 12 {
		bv := s[i : i+12]
		aid, err := BvToAV(bv)
		return InputID{AID: aid, BVID: bv, FileName: bv}, err
	}
	if i := strings.Index(strings.ToLower(s), "av"); i >= 0 {
		rest := s[i+2:]
		var digits strings.Builder
		for _, r := range rest {
			if r < '0' || r > '9' {
				break
			}
			digits.WriteRune(r)
		}
		if digits.Len() > 0 {
			aid, err := strconv.ParseInt(digits.String(), 10, 64)
			return InputID{AID: aid, BVID: AVToBV(aid), FileName: "av" + digits.String()}, err
		}
	}
	if aid, err := strconv.ParseInt(s, 10, 64); err == nil {
		return InputID{AID: aid, BVID: AVToBV(aid), FileName: "av" + s}, nil
	}
	return InputID{}, errors.New("could not find BV or av id")
}

func BvToAV(bv string) (int64, error) {
	if len(bv) != 12 || !strings.HasPrefix(bv, "BV1") {
		return 0, errors.New("invalid BV id")
	}
	digits := []byte(bv[3:])
	digits[0], digits[6] = digits[6], digits[0]
	digits[1], digits[4] = digits[4], digits[1]
	var avid int64
	for _, b := range digits {
		idx := strings.IndexByte(bvTable, b)
		if idx < 0 {
			return 0, errors.New("invalid BV id")
		}
		avid = avid*58 + int64(idx)
	}
	return (avid & bvMask) ^ bvXor, nil
}

func AVToBV(aid int64) string {
	x := (bvMax | aid) ^ bvXor
	out := make([]byte, 9)
	for i := len(out) - 1; x != 0 && i >= 0; i-- {
		out[i] = bvTable[x%58]
		x /= 58
	}
	out[0], out[6] = out[6], out[0]
	out[1], out[4] = out[4], out[1]
	return "BV1" + string(out)
}
