package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"time"
)

type Config struct {
	NodeUrl  string
	ApiKey   string
	AppPort  string
	BlockLag uint64
}

var EtherscanNetworks = map[uint64]string{
	1:      "api",
	3:      "api-ropsten",
	4:      "api-rinkeby",
	42:     "api-kovan",
	401697: "api-tobalaba",
}

const EtherscanAPIURL = "https://%s.etherscan.io/api?module=proxy&action=eth_blockNumber&apikey=%s"

const EnvHost = "NODE_HOST"
const EnvAppPort = "PORT"
const EnvEtherscanKey = "ETHERSCAN_API_KEY"

var client *ethclient.Client
var config Config
var netId uint64

func main() {
	var port = os.Getenv(EnvAppPort)
	if len(port) == 0 {
		port = "8080"
	}
	config = Config{
		ApiKey:  os.Getenv(EnvEtherscanKey),
		AppPort: port,
		NodeUrl: os.Getenv(EnvHost),
	}
	if len(config.NodeUrl) == 0 || len(config.ApiKey) == 0 {
		log.Fatal("Some ENV does not set")
	}
	ConnectToNode()
	http.HandleFunc("/read", CheckReadNode)
	http.HandleFunc("/live", CheckLiveNode)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		panic(err)
	}
}

func TryToConnect() {
	for i := 0; i < 5; i++ {
		isRun := ConnectToNode()
		if isRun {
			return
		}
		time.Sleep(2 * time.Second)
	}
	log.Println("WARN: Cannot connect to node")
}

func ConnectToNode() bool {
	_client, err := ethclient.Dial(config.NodeUrl)

	if err != nil {
		log.Println(err)
		return false
	} else {
		client = _client
		id, err := client.NetworkID(context.Background())
		if err != nil {
			log.Println(err)
			return false
		}
		netId = uint64(id.Uint64())
		return true
	}
}

func IsConnected() bool {
	if client == nil {
		return false
	}
	_, err := client.NetworkID(context.Background())
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}

func CheckReadNode(writer http.ResponseWriter, request *http.Request) {

	if !IsConnected() {
		TryToConnect()
	}

	sync, err := client.SyncProgress(context.TODO())

	if err != nil {
		log.Print(err)
		writer.WriteHeader(http.StatusServiceUnavailable)
		writer.Write([]byte(err.Error()))
		return
	}

	if sync != nil {
		log.Printf("Ethereum node is syncing now, current block: %d | highest block: %d", sync.CurrentBlock, sync.HighestBlock)
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	block, err := client.BlockByNumber(context.Background(), nil)

	if err != nil {
		log.Print(err)
		writer.WriteHeader(http.StatusServiceUnavailable)
		writer.Write([]byte(err.Error()))
		return
	}

	ethscanBlock := LoadBlockNumberEtherscan(netId)
	ethscanBlock = ethscanBlock.Sub(ethscanBlock, big.NewInt(5))

	if block.Number().Cmp(ethscanBlock) < 0 {
		log.Printf("Node is not valid, block %d but on Etherscan is %d", block.Number(), ethscanBlock)
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

func CheckLiveNode(writer http.ResponseWriter, request *http.Request) {

	if !IsConnected() {
		TryToConnect()
	}

	sync, err := client.SyncProgress(context.TODO())

	if err != nil {
		log.Print(err)
		writer.WriteHeader(http.StatusServiceUnavailable)
		writer.Write([]byte(err.Error()))
		return
	}

	block, err := client.BlockByNumber(context.Background(), nil)

	if err != nil {
		log.Print(err)
		writer.WriteHeader(http.StatusServiceUnavailable)
		writer.Write([]byte(err.Error()))
		return
	}

	ethscanBlock := LoadBlockNumberEtherscan(netId)
	ethscanBlock = ethscanBlock.Sub(ethscanBlock, big.NewInt(50))

	if sync == nil && block.Number().Cmp(ethscanBlock) < 0 {
		log.Print("Ethereum node is not actual but not syncing")
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

func LoadBlockNumberEtherscan(netId uint64) *big.Int {
	request := fmt.Sprintf(EtherscanAPIURL, EtherscanNetworks[netId], config.ApiKey)
	data, err := http.Get(request)
	if err != nil {
		log.Print(err)
		return big.NewInt(0)
	}
	defer data.Body.Close()
	var objmap map[string]*json.RawMessage
	body, err := ioutil.ReadAll(data.Body)
	if err != nil {
		log.Print(err)
		return big.NewInt(0)
	}
	err = json.Unmarshal(body, &objmap)
	if err != nil {
		log.Print(err)
		return big.NewInt(0)
	}
	var str string
	err = json.Unmarshal(*objmap["result"], &str)
	if err != nil {
		log.Print(err)
		return big.NewInt(0)
	}
	block := new(big.Int)
	block.SetString(str[2:], 16)
	return block
}
