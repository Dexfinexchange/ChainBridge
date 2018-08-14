package main

import (
	"fmt"
	//"net/http"
	"math/big"
	"io/ioutil"
	//"time"
	//"encoding/json"
	"encoding/hex"
	//"encoding/binary"
	"path/filepath"
	"strings"
	"log"
	"flag"
	"os"
	"sync"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/accounts/keystore"
    "github.com/ethereum/go-ethereum/accounts/abi"

    "github.com/ChainSafeSystems/multi_directional_bridge/client"
)

/* global vars */
var flags map[string]bool
var ks *keystore.KeyStore

/****** keystore methods ******/
func newKeyStore(path string) (*keystore.KeyStore) {
	newKeyStore := keystore.NewKeyStore(path, keystore.StandardScryptN, keystore.StandardScryptP)
	return newKeyStore
}

func readAbi(verbose bool) (*client.Events) {
	e := new(client.Events)

	// read bridge contract abi
	path, _ := filepath.Abs("./truffle/build/contracts/Bridge.json")
	file, err := ioutil.ReadFile(path)
	if err != nil {
	    fmt.Println("Failed to read file:", err)
	}

	fileAbi, err := client.ParseJsonForEntry(string(file), "abi")
	if err != nil {
		log.Fatal(err)
	}

	bridgeabi, err := abi.JSON(strings.NewReader(fileAbi))
	if err != nil {
	    fmt.Println("Invalid abi:", err)
	}

	// checking abi for events
	bridgeEvents := bridgeabi.Events
	depositEvent := bridgeEvents["Deposit"]
	depositHash := depositEvent.Id()
	e.DepositId = depositHash.Hex()
	if verbose { fmt.Println("deposit event id: ", e.DepositId) }

	creationEvent := bridgeEvents["ContractCreation"]
	creationHash := creationEvent.Id()
	e.CreationId = creationHash.Hex()
	if verbose { fmt.Println("contract creation event id: ", e.CreationId) }

	withdrawEvent := bridgeEvents["Withdraw"]
	withdrawHash := withdrawEvent.Id()
	e.WithdrawId = withdrawHash.Hex()
	if verbose { fmt.Println("withdraw event id: ", e.WithdrawId) }

	bridgeSetEvent := bridgeEvents["BridgeSet"]
	bridgeSetHash := bridgeSetEvent.Id()
	e.BridgeSetId = bridgeSetHash.Hex()
	if verbose { fmt.Println("set bridge event id: ", e.BridgeSetId) }

	bridgeFundedEvent := bridgeEvents["BridgeFunded"]
	bridgeFundedHash := bridgeFundedEvent.Id()
	e.BridgeFundedId = bridgeFundedHash.Hex()
	if verbose { fmt.Println("bridge funded event id: ", e.BridgeFundedId) }

	paidEvent := bridgeEvents["Paid"]
	paidHash := paidEvent.Id()
	e.PaidId = paidHash.Hex()
	if verbose { fmt.Println("bridge paid event id", e.PaidId) }
	return e
}

func startup(id *big.Int) (*big.Int) {
	path, _ := filepath.Abs("./log/" + id.String() + "_lastblock.txt")
	file, err := ioutil.ReadFile(path)
	if err != nil {
	    fmt.Println("Failed to read file:", err)
	}
	startBlock := new(big.Int)
	startBlock.SetString(string(file), 10)
	return startBlock
}

func printHeader() {
	fmt.Println("██████╗ ██████╗ ██╗██████╗  ██████╗ ███████╗")
	fmt.Println("██╔══██╗██╔══██╗██║██╔══██╗██╔════╝ ██╔════╝")
	fmt.Println("██████╔╝██████╔╝██║██║  ██║██║  ███╗█████╗  ")
	fmt.Println("██╔══██╗██╔══██╗██║██║  ██║██║   ██║██╔══╝  ")
	fmt.Println("██████╔╝██║  ██║██║██████╔╝╚██████╔╝███████╗")
	fmt.Println("╚═════╝ ╚═╝  ╚═╝╚═╝╚═════╝  ╚═════╝ ╚══════╝")
}

func main() {
	//printHeader()
	// var input string
	// fmt.Println("enter function: ")
	// fmt.Scanln(&input)
	// sig := function.GenerateSignature(input)
	// fmt.Println(sig)

	/* flags */
	headerPtr := flag.Bool("header", true, "a bool representing whether to print out the header or not")
	verbosePtr := flag.Bool("v", false, "a bool representing verbosity of output")
	readAllPtr := flag.Bool("a", false, "a bool representing whether to read logs from every contract or not")
	configPtr := flag.String("config", "./config.json", "a string of the path to the config file") 
	keysPtr := flag.String("keystore", "./keystore", "a string of the path to the keystore directory") 
	// password flag assumes you have the same account on every chain
	passwordPtr := flag.String("password", "password", "a string of the password to the account specified in the config file") 
	noListenPtr := flag.Bool("no-listen", false, "a bool; if true, do not start the listener")

	depositCommand := flag.NewFlagSet("deposit", flag.ExitOnError)
	fundCommand := flag.NewFlagSet("fund", flag.ExitOnError)
	payCommand := flag.NewFlagSet("payCommand", flag.ExitOnError)
	withdrawCommand := flag.NewFlagSet("withrawCommand", flag.ExitOnError)

	// subcommands
	// ./b
	if len(os.Args) > 1 {
		switch os.Args[1]{
			case "deposit":
				depositCommand.Parse(os.Args[2:])
			case "fund":
				fundCommand.Parse(os.Args[2:])
			case "pay":
				payCommand.Parse(os.Args[2:])
			case "withdraw":
				withdrawCommand.Parse(os.Args[2:])
			default:
				// continue
		}
	}

	flag.Parse()
	header := *headerPtr
	if header { printHeader() }

	configStr := *configPtr
	fmt.Println("config path: ", configStr)

	verbose := *verbosePtr
	if verbose { fmt.Println("verbose: ", verbose) }

	readAll := *readAllPtr
	if readAll { fmt.Println("read from all contracts? ", readAll)}

	keystorePath := *keysPtr
	fmt.Println("keystore path: ", keystorePath)

	password := *passwordPtr
	noListen := *noListenPtr

	var chains []string
	if depositCommand.Parsed() {
		chains = depositCommand.Args()
		fmt.Println("deposit to:", chains)
	} else if fundCommand.Parsed() {
		chains = fundCommand.Args()
		fmt.Println("fund bridge on chains", chains)
	} else if payCommand.Parsed() {
		chains = payCommand.Args()
		fmt.Println("pay bridge on chains", chains)
	} else if withdrawCommand.Parsed() {
		chains = withdrawCommand.Args()
		fmt.Println("withdraw from bridge on chains", chains)
	} else {
		chains = flag.Args()
		if len(chains) == 0 {
			chains = append(chains,"1")
		}
		fmt.Println("chains to connect to: ", chains)
	}

	flags = make(map[string]bool)
	flags["v"] = verbose
	flags["a"] = readAll
	flags["nolisten"] = noListen

	/* keys */
	ks = newKeyStore(keystorePath)
	ksaccounts := ks.Accounts()
	for i, account := range ksaccounts {
		if verbose { fmt.Println("account", i, ":", account.Address.Hex()) }
	}

	// config file reading
	path, _ := filepath.Abs(configStr)
	file, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println("Failed to read file:", err)	
	}

	clients := make([]*client.Chain, len(chains))
	// read config file for each chain id
	for i, chain := range chains {
		clients[i] = new(client.Chain)

		clients[i].Id = new(big.Int)
		clients[i].Id.SetString(chain, 10)

		// to start at block 0, `rm -rf log/`
		startBlock := startup(clients[i].Id)
		clients[i].StartBlock = startBlock

		chainStr, err := client.ParseJsonForEntry(string(file), chain)
		if err != nil {
			fmt.Println("could not find chain in config file")
			log.Fatal(err)
		}

		contractAddr, err := client.ParseJsonForEntry(chainStr, "contractAddr")
		if err != nil {
			fmt.Println("could not find contractAddr in config file")
			log.Fatal(err)
		}
		fmt.Println("contract address of chain", chain, ":", contractAddr)
		contract := new(common.Address)
		contractBytes, err := hex.DecodeString(contractAddr[2:])
		if err != nil {
			log.Fatal(err)
		}
		contract.SetBytes(contractBytes)
		clients[i].Contract = contract

		url, err := client.ParseJsonForEntry(chainStr, "url")
		if err != nil {
			fmt.Println("could not find url in config file")
			log.Fatal(err)
		}
		fmt.Println("url of chain", chain, ":", url)
		clients[i].Url = url

		gp, err := client.ParseJsonForEntry(chainStr, "gasPrice")
		if err != nil {
			fmt.Println("could not find gas price in config file")
			log.Fatal(err)
		}
		bigGas := new(big.Int)
		bigGas.SetString(gp, 10)
		clients[i].GasPrice = bigGas

		fromAccount, err := client.ParseJsonForEntry(chainStr, "from")
		if err != nil {
			fmt.Println("could not find from account in config file")
			log.Fatal(err)
		}
		fmt.Println("account to send txs from on chain", chain, ":", fromAccount)
		from := new(common.Address)
		fromBytes, err := hex.DecodeString(fromAccount[2:])
		if err != nil {
			log.Fatal(err)
		}
		from.SetBytes(fromBytes)
		clients[i].From = from

		clients[i].Password = password

		/* unlock account */
		// if(ks.HasAddress(*from)) {
		// 	account := new(accounts.Account)
		// 	account.Address = *from
		// 	err = ks.Unlock(*account, password)
		// 	if err != nil {
		// 		fmt.Println("could not unlock account")
		// 		fmt.Println(err)
		// 	} else {
		// 		log.Fatal("account not found in keystore")
		// 	}
		// }
	}

	for _, chain := range clients {
		/* dial client */
		chainClient, err := ethclient.Dial(chain.Url)
		if err != nil {
			log.Fatal(err)
		}
		chain.Client = chainClient
	}

	/* read abi of contract in truffle folder */
	events := readAbi(flags["v"])

	if depositCommand.Parsed() {
		for _, chain := range chains {
			id, err := new(big.Int).SetString(chain, 10)
			if err != true {
				log.Fatal("could not find chain", chain)
			}
			chain := client.FindChain(id, clients)
			client.DepositPrompt(chain, ks)
		}
		return
	} else if fundCommand.Parsed() {
		for _, chain := range chains {
			id, err := new(big.Int).SetString(chain, 10)
			if err != true {
				log.Fatal("could not find chain", chain)
			}
			chain := client.FindChain(id, clients)
			client.FundPrompt(chain, ks)
		}
		return
	} else if payCommand.Parsed() {
		for _, chain := range chains {
			id, err := new(big.Int).SetString(chain, 10)
			if err != true {
				log.Fatal("could not find chain", chain)
			}
			chain := client.FindChain(id, clients)
			client.PayBridgePrompt(chain, ks)
		}
		return
	} else if  withdrawCommand.Parsed() {
		for _, chain := range chains {
			id, err := new(big.Int).SetString(chain, 10)
			if err != true {
				log.Fatal("could not find chain", chain)
			}
			chain := client.FindChain(id, clients)
			client.WithdrawToPrompt(chain, ks)
		}
		return	
	}

	/* channels */
	doneClient := make(chan bool)

	// wait group for interrupt handling
	wg := new(sync.WaitGroup)
	wg.Add(len(clients))

	if(!noListen) {
		/* listener */
		fmt.Println("\nlistening for events...")
		for _, chain := range clients {
			go client.Listen(chain, clients, events, doneClient, ks, flags, wg)
		}

		<-doneClient
	}
}