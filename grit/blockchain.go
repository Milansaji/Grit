package grit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
)

// Blockchain instance
var ethRPCURL string

// ConnectEthereum initializes the connection to an Ethereum node.
func ConnectEthereum(rpcURL string) error {
	ethRPCURL = rpcURL
	// Test connection with a simple call
	_, err := GetEthBalance("0x0000000000000000000000000000000000000000")
	if err != nil {
		return fmt.Errorf("failed to connect to Ethereum RPC: %v", err)
	}
	return nil
}

// GetEthBalance returns the balance of an address in Wei using JSON-RPC.
func GetEthBalance(address string) (*big.Int, error) {
	if ethRPCURL == "" {
		return nil, fmt.Errorf("ethereum RPC URL not set. Call EthConnect() first")
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_getBalance",
		"params":  []interface{}{address, "latest"},
		"id":      1,
	}

	respBody, err := callRPC(payload)
	if err != nil {
		return nil, err
	}

	var result struct {
		Result string `json:"result"`
		Error  struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	if result.Error.Message != "" {
		return nil, fmt.Errorf("RPC error: %s", result.Error.Message)
	}

	// Robust Hex to big.Int parsing
	hexVal := result.Result
	if len(hexVal) >= 2 && hexVal[:2] == "0x" {
		hexVal = hexVal[2:]
	}
	if hexVal == "" {
		hexVal = "0"
	}

	balance := new(big.Int)
	_, ok := balance.SetString(hexVal, 16)
	if !ok {
		return nil, fmt.Errorf("failed to parse balance hex: %s", result.Result)
	}

	return balance, nil
}

// TransactContract (Simplified placeholder for lightweight version)
func TransactContract(privateKeyHex, contractAddr, abiJSON, method string, args ...interface{}) (string, error) {
	return "", fmt.Errorf("writing transactions requires a full Web3 library (go-ethereum). For now, use Grit for reading blockchain data")
}

func callRPC(payload interface{}) ([]byte, error) {
	data, _ := json.Marshal(payload)
	resp, err := http.Post(ethRPCURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
