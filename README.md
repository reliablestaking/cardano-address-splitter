# Description

This program will look for transations sent to a given address, and when enough UTXOs are in the address, it will split them to two seperate address. (20% to one and 80% to the other)

# Prerequisites

1. golang (1.18.3 or higher)
2. cardano-cli
3. Blockfrost Account (https://blockfrost.io/)

# Setup

1. Generate keys and address in keys directory

```
cardano-cli address key-gen --verification-key-file keys/payment.vkey --signing-key-file keys/payment.skey
cardano-cli address build --payment-verification-key-file keys/payment.vkey --out-file keys/payment.addr --mainnet
```

*NOTE: BACKUP THE PAYMENT.SKEY, without this the funds will be lost forever.

2. Build program by running `go build`

3. Configure shell script (splitter.sh) and run it

4. Optionally, setup to run as a systemd service on linux

# Env Config

```
# Address to monitor
MONITOR_ADDRESS=
# How many utxo to accumulate before splitting
UTXO_COUNT_SPLIT=
# API Key for blockfrost
BLOCKFROST_PROJECT_ID=
# Send 20% to this address
SPLIT_LOWER_ADDRESS=
# Send 80% to this address
SPLIT_HIGHER_ADDRESS=
```