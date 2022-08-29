package cardanocli

import (
	"strconv"
	"strings"

	"github.com/reliablestaking/cardano-address-splitter/environment"
	"github.com/sirupsen/logrus"
)

func BuildTransaction(fileName string, txsIn []string, txsOut []string, ttl int, fee int) error {
	logrus.Infof("Building draft transaction with %d in and %d out", len(txsIn), len(txsOut))

	args := make([]string, 0)
	args = append(args, "transaction")
	args = append(args, "build-raw")

	for _, txIn := range txsIn {
		args = append(args, "--tx-in")
		args = append(args, txIn)
	}
	for _, txOut := range txsOut {
		args = append(args, "--tx-out")
		args = append(args, txOut)
	}
	args = append(args, "--invalid-hereafter")
	args = append(args, strconv.Itoa(ttl))
	args = append(args, "--fee")
	args = append(args, strconv.Itoa(fee))
	args = append(args, "--out-file")
	args = append(args, fileName)

	_, err := environment.RunCommand("cardano-cli", args...)
	if err != nil {
		return err
	}

	return nil
}

func CalculateFee(txDraftFile string, txInCount int, txOutCount int, witnessCount int) (int, error) {
	logrus.Info("Building draft transaction")

	args := make([]string, 0)
	args = append(args, "transaction")
	args = append(args, "calculate-min-fee")
	args = append(args, "--tx-body-file")
	args = append(args, txDraftFile)
	args = append(args, "--tx-in-count")
	args = append(args, strconv.Itoa(txInCount))
	args = append(args, "--tx-out-count")
	args = append(args, strconv.Itoa(txOutCount))
	args = append(args, "--witness-count")
	args = append(args, strconv.Itoa(witnessCount))
	args = append(args, "--byron-witness-count")
	args = append(args, "0")
	args = append(args, "--protocol-params-file")
	args = append(args, "keys/protocol-params.json")

	output, err := environment.RunCommand("cardano-cli", args...)
	if err != nil {
		return 0, err
	}

	fee := output[0][0:strings.Index(output[0], " ")]
	feeInt, err := strconv.Atoi(fee)
	if err != nil {
		return 0, err
	}

	return feeInt, nil
}

func SignTransaction(txFile string, signingKey1 string, signingKey2 string, outFile string) error {
	logrus.Info("Signing transaction")

	args := make([]string, 0)
	args = append(args, "transaction")
	args = append(args, "sign")
	args = append(args, "--tx-body-file")
	args = append(args, txFile)
	if signingKey1 != "" {
		args = append(args, "--signing-key-file")
		args = append(args, signingKey1)
	}
	if signingKey2 != "" {
		args = append(args, "--signing-key-file")
		args = append(args, signingKey2)
	}
	args = append(args, "--out-file")
	args = append(args, outFile)
	args = append(args, "--mainnet")

	_, err := environment.RunCommand("cardano-cli", args...)
	if err != nil {
		return err
	}

	return nil
}

func BuildRawFile(signedFile string, cborHex string) (string, error) {
	outputFilename := signedFile + ".bin"

	_, err := environment.RunCommand("touch " + "file.test")
	if err != nil {
		return "", err
	}
	args := make([]string, 0)

	_, err = environment.RunCommand("xxd -r -p <<< "+cborHex+" > "+outputFilename, args...)
	if err != nil {
		return "", err
	}

	return outputFilename, nil
}
