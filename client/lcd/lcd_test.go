package lcd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	keys "github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cryptoKeys "github.com/tendermint/go-crypto/keys"
	"github.com/tendermint/tendermint/p2p"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

func TestKeys(t *testing.T) {
	kill, port, _ := junkInit(t)
	defer kill()

	// empty keys
	res, body := request(t, port, "GET", "/keys", nil)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	assert.Equal(t, "[]", body, "Expected an empty array")

	// get seed
	res, body = request(t, port, "GET", "/keys/seed", nil)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	seed := body
	reg, err := regexp.Compile(`([a-z]+ ){12}`)
	require.Nil(t, err)
	match := reg.MatchString(seed)
	assert.True(t, match, "Returned seed has wrong foramt", seed)

	// add key
	var jsonStr = []byte(`{"name":"test_fail", "password":"1234567890"}`)
	res, body = request(t, port, "POST", "/keys", jsonStr)

	assert.Equal(t, http.StatusBadRequest, res.StatusCode, "Account creation should require a seed")

	jsonStr = []byte(fmt.Sprintf(`{"name":"test", "password":"1234567890", "seed": "%s"}`, seed))
	res, body = request(t, port, "POST", "/keys", jsonStr)

	assert.Equal(t, http.StatusOK, res.StatusCode, body)
	addr := body
	assert.Len(t, addr, 40, "Returned address has wrong format", addr)

	// existing keys
	res, body = request(t, port, "GET", "/keys", nil)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var m [1]keys.KeyOutput
	err = json.Unmarshal([]byte(body), &m)
	require.Nil(t, err)

	assert.Equal(t, m[0].Name, "test", "Did not serve keys name correctly")
	assert.Equal(t, m[0].Address, addr, "Did not serve keys Address correctly")

	// select key
	res, body = request(t, port, "GET", "/keys/test", nil)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var m2 keys.KeyOutput
	err = json.Unmarshal([]byte(body), &m2)
	require.Nil(t, err)

	assert.Equal(t, "test", m2.Name, "Did not serve keys name correctly")
	assert.Equal(t, addr, m2.Address, "Did not serve keys Address correctly")

	// update key
	jsonStr = []byte(`{"old_password":"1234567890", "new_password":"12345678901"}`)
	res, body = request(t, port, "PUT", "/keys/test", jsonStr)
	require.Equal(t, http.StatusOK, res.StatusCode, body)

	// here it should say unauthorized as we changed the password before
	res, body = request(t, port, "PUT", "/keys/test", jsonStr)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode, body)

	// delete key
	jsonStr = []byte(`{"password":"12345678901"}`)
	res, body = request(t, port, "DELETE", "/keys/test", jsonStr)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
}

func TestVersion(t *testing.T) {
	kill, port, _ := junkInit(t)
	defer kill()

	// node info
	res, body := request(t, port, "GET", "/version", nil)
	require.Equal(t, http.StatusOK, res.StatusCode, body)

	reg, err := regexp.Compile(`\d+\.\d+\.\d+(-dev)?`)
	require.Nil(t, err)
	match := reg.MatchString(body)
	assert.True(t, match, body)
}

func TestNodeStatus(t *testing.T) {
	kill, port, _ := junkInit(t)
	defer kill()

	// node info
	res, body := request(t, port, "GET", "/node_info", nil)
	require.Equal(t, http.StatusOK, res.StatusCode, body)

	var nodeInfo p2p.NodeInfo
	err := json.Unmarshal([]byte(body), &nodeInfo)
	require.Nil(t, err, "Couldn't parse node info")

	assert.NotEqual(t, p2p.NodeInfo{}, nodeInfo, "res: %v", res)

	// syncing
	res, body = request(t, port, "GET", "/syncing", nil)
	require.Equal(t, http.StatusOK, res.StatusCode, body)

	assert.Equal(t, "true", body)
}

func TestBlock(t *testing.T) {
	kill, port, _ := junkInit(t)
	defer kill()

	time.Sleep(time.Second * 2) // TODO: LOL -> wait for blocks

	var resultBlock ctypes.ResultBlock

	res, body := request(t, port, "GET", "/blocks/latest", nil)
	require.Equal(t, http.StatusOK, res.StatusCode, body)

	err := json.Unmarshal([]byte(body), &resultBlock)
	require.Nil(t, err, "Couldn't parse block")

	assert.NotEqual(t, ctypes.ResultBlock{}, resultBlock)

	// --

	res, body = request(t, port, "GET", "/blocks/1", nil)
	require.Equal(t, http.StatusOK, res.StatusCode, body)

	err = json.Unmarshal([]byte(body), &resultBlock)
	require.Nil(t, err, "Couldn't parse block")

	assert.NotEqual(t, ctypes.ResultBlock{}, resultBlock)

	// --

	res, body = request(t, port, "GET", "/blocks/1000000000", nil)
	require.Equal(t, http.StatusNotFound, res.StatusCode, body)
}

func TestValidators(t *testing.T) {
	kill, port, _ := junkInit(t)
	defer kill()

	time.Sleep(time.Second * 2) // TODO: LOL -> wait for blocks

	var resultVals ctypes.ResultValidators

	res, body := request(t, port, "GET", "/validatorsets/latest", nil)
	require.Equal(t, http.StatusOK, res.StatusCode, body)

	err := json.Unmarshal([]byte(body), &resultVals)
	require.Nil(t, err, "Couldn't parse validatorset")

	assert.NotEqual(t, ctypes.ResultValidators{}, resultVals)

	// --

	res, body = request(t, port, "GET", "/validatorsets/1", nil)
	require.Equal(t, http.StatusOK, res.StatusCode, body)

	err = json.Unmarshal([]byte(body), &resultVals)
	require.Nil(t, err, "Couldn't parse validatorset")

	assert.NotEqual(t, ctypes.ResultValidators{}, resultVals)

	// --

	res, body = request(t, port, "GET", "/validatorsets/1000000000", nil)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestCoinSend(t *testing.T) {
	kill, port, seed := junkInit(t)
	defer kill()

	time.Sleep(time.Second * 2) // TO

	// query empty
	res, body := request(t, port, "GET", "/accounts/8FA6AB57AD6870F6B5B2E57735F38F2F30E73CB6", nil)
	require.Equal(t, http.StatusNoContent, res.StatusCode, body)

	// create account from seed who has keys
	var jsonStr = []byte(fmt.Sprintf(`{"name":"account_with_coins", "password":"1234567890", "seed": "%s"}`, seed))
	res, body = request(t, port, "POST", "/keys", jsonStr)

	assert.Equal(t, http.StatusOK, res.StatusCode, body)
	addr := body

	// query
	res, body = request(t, port, "GET", "/accounts/"+addr, nil)
	require.Equal(t, http.StatusOK, res.StatusCode, body)

	assert.Equal(t, `{
		"coins": [
			{
				"denom": "mycoin",
				"amount": 9007199254740992
			}
		]
	}`, body)

	// create receive address
	kb := client.MockKeyBase()
	receiveInfo, _, err := kb.Create("receive_address", "1234567890", cryptoKeys.CryptoAlgo("ed25519"))
	require.Nil(t, err)
	receiveAddr := string(receiveInfo.Address())

	// send
	jsonStr = []byte(`{
		"name":"account_with_coins", 
		"password":"1234567890", 
		"amount":[{
			"denom": "mycoin",
			"amount": 1
		}]
	}`)
	res, body = request(t, port, "POST", "/accounts/"+receiveAddr+"/send", jsonStr)
	require.Equal(t, http.StatusOK, res.StatusCode, body)

	// check if received
	res, body = request(t, port, "GET", "/accounts/"+receiveAddr, nil)
	require.Equal(t, http.StatusOK, res.StatusCode, body)

	assert.Equal(t, `{
		"coins": [
			{
				"denom": "mycoin",
				"amount": 1
			}
		]
	}`, body)
}

//__________________________________________________________
// helpers

// TODO/XXX: We should be spawning what we need in process, not shelling out
func junkInit(t *testing.T) (kill func(), port string, seed string) {
	dir, err := ioutil.TempDir("", "tmp-basecoin-")
	require.Nil(t, err)

	seed = tests.TestInitBasecoin(t, dir)
	cmdStart := tests.StartNodeServerForTest(t, dir)
	cmdLCD, port := tests.StartLCDServerForTest(t, dir)
	kill = func() {
		cmdLCD.Process.Kill()
		cmdStart.Process.Kill()
		os.Remove(dir)
	}
	return kill, port, seed
}

func request(t *testing.T, port, method, path string, payload []byte) (*http.Response, string) {
	var res *http.Response
	var err error
	url := fmt.Sprintf("http://localhost:%v%v", port, path)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(payload))
	require.Nil(t, err)
	res, err = http.DefaultClient.Do(req)
	//	res, err = http.Post(url, "application/json", bytes.NewBuffer(payload))
	require.Nil(t, err)

	output, err := ioutil.ReadAll(res.Body)
	require.Nil(t, err)

	return res, string(output)
}