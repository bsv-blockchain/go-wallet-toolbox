# BSV Faucet Server - Quick Start

## 🚀 Deploy in 3 Steps

### 1. Setup
```bash
# Install Docker
sudo apt update && sudo apt install -y docker.io docker-compose

# Clone and configure
git clone https://github.com/bsv-blockchain/go-wallet-toolbox.git
cd go-wallet-toolbox/examples/complex_wallet_examples/create_faucet_server
cp .env.example .env
nano .env  # Add your private keys
```

### 2. Deploy
```bash
docker-compose up -d --build
```

### 3. Verify
```bash
curl http://localhost:8080/info
```

## 📋 Environment Variables

Edit `.env` file:
```bash
FAUCET_PRIVATE_KEY=your_faucet_private_key_here
NETWORK=test  # or main
PORT=8080
# Optional: set a cap on the total faucet amount per request (satoshis)
MAX_FAUCET_TOTAL_AMOUNT=20000
```

## 📊 API Endpoints

### `GET /info`
Get faucet address, balance and network.

**Response:**
```json
{
  "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
  "balance": 1000000,
  "network": "test"
}
```

### `POST /faucet`
Send funds to one or more addresses.

**Request:**
```json
{
  "outputs": [
    { "address": "...", "amount": 1000 },
    { "address": "...", "amount": 2000 }
  ]
}
```

**Response:**
```json
{ "status": "ok", "message": "funded", "txid": "abc123...", "beef_hex": "0100..." }
```

### `POST /topup`
Add funds to faucet by internalizing a UTXO.

**Request:**
```json
{ "outpoint": "txid:outputIndex" }
```

**Response:**
```json
{ "status": "ok" }
```

## 🔧 Useful Commands

```bash
# Start/Stop
docker-compose up -d
docker-compose down

# Logs
docker-compose logs -f faucet-server

# Status
docker-compose ps

# Update
git pull && docker-compose up -d --build
```
