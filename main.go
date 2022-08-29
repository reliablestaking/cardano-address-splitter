package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/blockfrost/blockfrost-go"
	"github.com/google/uuid"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"

	cli "github.com/reliablestaking/cardano-address-splitter/cardanocli"
)

type (
	SignedTx struct {
		Hex string `json:"cborHex"`
	}
)

func main() {
	logrus.Info("Starting Cardano Address Splitter...")

	// get address to monitor
	addressToMonitor := os.Getenv("MONITOR_ADDRESS")
	if addressToMonitor == "" {
		logrus.Fatal("No address configured to monitor")
	}

	// init blockfrost api
	api := blockfrost.NewAPIClient(
		blockfrost.APIClientOptions{Server: "https://cardano-preprod.blockfrost.io/api/v0/"},
	)

	// get api info to validate configured correctly
	info, err := api.Info(context.TODO())
	if err != nil {
		logrus.WithError(err).Fatal("Error getting api info")
	}

	// get address to send to
	splitLowerAddress := os.Getenv("SPLIT_LOWER_ADDRESS")
	if splitLowerAddress == "" {
		logrus.Fatal("No split lower address configured")
	}
	splitHigherAddress := os.Getenv("SPLIT_HIGHER_ADDRESS")
	if splitHigherAddress == "" {
		logrus.Fatal("No split higher address configured")
	}

	// get utxos
	utxoCountString := os.Getenv("UTXO_COUNT_SPLIT")
	utxoCount, err := strconv.Atoi(utxoCountString)
	if err != nil {
		logrus.WithError(err).Fatalf("Error parsing split into int %s", utxoCountString)
	}

	logrus.Infof("API Info: tUrl: %s Version: %s", info.Url, info.Version)

	for {
		logrus.Info("Running address check on %s", addressToMonitor)

		addressUtxos, err := api.AddressUTXOs(context.Background(), addressToMonitor, blockfrost.APIQueryParams{})
		if err != nil {
			logrus.WithError(err).Error("Error getting address utxos")
		} else {
			logrus.Infof("Found %d utxos for address", len(addressUtxos))

			// if enough utxos, then split
			if len(addressUtxos) >= utxoCount {
				// only send the count number, even if more
				utxoToSend := make([]blockfrost.AddressUTXO, 0)
				if len(addressUtxos) > utxoCount {
					utxoToSend = append(utxoToSend, addressUtxos[0:utxoCount]...)
				} else {
					utxoToSend = append(utxoToSend, addressUtxos...)
				}

				err = SplitUtxos(addressUtxos, splitLowerAddress, splitHigherAddress, api)
				if err != nil {
					logrus.WithError(err).Error("Error building tx")
				}
			}
		}

		//TODO: mainnet protocol params
		time.Sleep(60 * time.Minute)
	}

}

func SplitUtxos(utxos []blockfrost.AddressUTXO, splitLowerAddress, splitHigherAddress string, api blockfrost.APIClient) error {
	// makedir for tx files
	dirName := uuid.New().String()
	err := os.Mkdir(dirName, 0755)
	if err != nil {
		logrus.WithError(err).Errorf("Error creating directory")
		return err
	}

	txsIn := make([]string, 0)
	txsOut := make([]string, 0)
	outputAmount := 0
	for _, utxo := range utxos {
		//build txIn
		txIn := fmt.Sprintf("%s#%d", utxo.TxHash, utxo.OutputIndex)
		txsIn = append(txsIn, txIn)
		for _, amount := range utxo.Amount {
			quantity, err := strconv.Atoi(amount.Quantity)
			if err != nil {
				logrus.WithError(err).Error("Error converting quantity to int")
				return err
			}

			outputAmount += quantity
		}
	}

	// build tx out
	splitLower := float64(outputAmount) * 0.2
	splitLowerInt := int(splitLower)

	returnTxOutLower := fmt.Sprintf("%s+%d", splitLowerAddress, splitLowerInt)
	txsOut = append(txsOut, returnTxOutLower)
	returnTxOutHigher := fmt.Sprintf("%s+%d", splitHigherAddress, outputAmount-splitLowerInt)
	txsOut = append(txsOut, returnTxOutHigher)

	draftTxFile := fmt.Sprintf("%s/%s", dirName, "tx.draft")
	err = cli.BuildTransaction(draftTxFile, txsIn, txsOut, 0, 0)
	if err != nil {
		logrus.WithError(err).Errorf("Error building draft transaction")
		return err
	}
	fee, err := cli.CalculateFee(draftTxFile, len(txsIn), len(txsOut), 1)
	if err != nil {
		logrus.WithError(err).Errorf("Error calculating fee")
		return err
	}
	logrus.Infof("Calculated a fee of %d", fee)

	//incorporate fee
	txsOut = make([]string, 0)
	returnTxOutLower = fmt.Sprintf("%s+%d", splitLowerAddress, splitLowerInt)
	txsOut = append(txsOut, returnTxOutLower)
	//TODO: also split fee
	returnTxOutHigher = fmt.Sprintf("%s+%d", splitHigherAddress, outputAmount-splitLowerInt-fee)
	txsOut = append(txsOut, returnTxOutHigher)

	// get ttl
	block, err := api.BlockLatest(context.Background())
	if err != nil {
		logrus.WithError(err).Errorf("Error getting latet block")
		return err
	}
	logrus.Infof("Found slot of %d", block.Slot)

	// build actual transaction
	actualTxFile := fmt.Sprintf("%s/%s", dirName, "split.tx")

	//vaid for 1 hour
	err = cli.BuildTransaction(actualTxFile, txsIn, txsOut, block.Slot+10800, fee)
	if err != nil {
		logrus.WithError(err).Errorf("Error building transaction")
		return err
	}

	// sign file
	signedTxFile := fmt.Sprintf("%s/%s", dirName, "mint.signed")

	err = cli.SignTransaction(actualTxFile, "keys/payment.skey", "", signedTxFile)
	if err != nil {
		logrus.WithError(err).Errorf("Error signing transaction")
		return err
	}

	// get cbor hex
	jsonFile, err := os.Open(signedTxFile)
	if err != nil {
		logrus.WithError(err).Errorf("Error opening file")
		return err
	}

	byteValue, _ := ioutil.ReadAll(jsonFile)
	var signedTx SignedTx
	json.Unmarshal(byteValue, &signedTx)

	txHash := ""
	txHex := ""
	//submit transaction
	logrus.Info("Submitting tx using blockfrost")
	data, err := hex.DecodeString(signedTx.Hex)
	if err != nil {
		panic(err)
	}

	txHex, err = api.TransactionSubmit(context.Background(), data)

	logrus.Infof("Submitted tx: %s", txHex)
	if err != nil {
		logrus.WithError(err).Errorf("Error submitting tx")
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	txHash = strings.ReplaceAll(txHex, "\"", "")

	//check if tx successfull
	if txHash != "" {
		for i := 0; i < 30; i++ {
			transaction, err := api.Transaction(context.Background(), txHash)
			if err != nil {
				logrus.WithError(err).Errorf("Error verifying tx, trying again...")
			}

			if transaction.Hash != "" {
				logrus.Infof("Transaction %s found", transaction.Hash)
				break
			}

			time.Sleep(time.Second * 5)
		}
	}

	jsonFile.Close()
	//remove folder
	err = os.RemoveAll(dirName)
	if err != nil {
		logrus.WithError(err).Errorf("Error removing directory")
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return nil
}
