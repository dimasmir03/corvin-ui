package utils

import (
	"crypto/sha1"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"github.com/google/uuid"
)

type VlessParams struct {
	Link string
	UID  string
	PBK  string
	SID  string
	SPX  string
	Flow string
}

var rend *rand.Rand

func GenVlessLink(tgID int64) VlessParams {
	rand.New(rand.NewSource(time.Now().UnixNano()))

	// ----------- GENERATE NAME FROM tgID -----------
	// h := sha1.Sum([]byte(strconv.FormatInt(tgID, 10)))
	h := sha1.Sum([]byte(uuid.New().String() + time.Now().String()))
	name := fmt.Sprintf("vp-%x", h[:8]) // 10 hex chars (5 bytes)
	//

	uid := uuid.New().String()
	pbk := "sompOjrok5Nr0zdcLcgFKdE98YJFb0GthGkRUyaleXs"
	sids := []string{"fd6546ec484b44", "4297", "f0f8698d", "157dae", "997b2ad79c", "3edb7ff0ea3a2696", "ecfcb9651147", "e5"}
	sid := sids[rand.Intn(len(sids))]
	spx := "/"
	snis := []string{"yahoo.com"}
	sni := snis[rand.Intn(len(snis))]

	ip := "raven.net.ru" // может быть заменен на проксируемый домен или IP

	u := url.URL{
		Scheme: "vless",
		User:   url.User(uid),
		Host:   fmt.Sprintf("%s:443", ip),
		RawQuery: url.Values{
			"type":     []string{"tcp"},
			"security": []string{"reality"},
			"pbk":      []string{pbk},
			"fp":       []string{"chrome"},
			"sni":      []string{sni},
			"sid":      []string{sid},
			"spx":      []string{spx},
			"flow":     []string{"xtls-rprx-vision"},
		}.Encode(),
		Fragment: name,
	}

	fmt.Println(u.User)

	return VlessParams{
		Link: u.String(),
		UID:  uid,
		PBK:  pbk,
		SID:  sid,
		SPX:  spx,
		Flow: "xtls-rprx-vision",
	}
}

func randomHex(n int) string {
	letters := []rune("abcdef0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func randomString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
