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
curl http://localhost:8080/address
```

## 📋 Environment Variables

Edit `.env` file:
```bash
SERVER_PRIVATE_KEY=your_server_private_key_here
FAUCET_PRIVATE_KEY=your_faucet_private_key_here
NETWORK=test  # or main
PORT=8080
SERVER_URL=http://127.0.0.1:8100
```

## 📊 API Endpoints

### `GET /address`
Get faucet address and balance.

**Response:**
```json
{
  "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
  "balance": 1000000
}
```

### `POST /faucet`
Send funds to one or more addresses.

**Request:**
```json
{
  "outputs": [
    {
      "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
      "amount": 1000
    },
    {
      "address": "1B2M2Y8AsgTpgAmY7PhCfgQggdL41VXaC",
      "amount": 2000
    }
  ]
}
```

**Response:**
```json
{
  "status": "ok",
  "message": "funded",
  "txid": "abc123...",
  "beef_hex": "01000000..."
}
```

**Limits:**
- Maximum total amount: **20,000 satoshis** per request (configurable)
- At least one output required
- Amount must be > 0 for each output

### `POST /topup`
Add funds to faucet by internalizing a UTXO.

**Request:**
```json
{
  "outpoint": "txid:outputIndex"
}
```

**Example:**
```json
{
  "outpoint": "abc123def456789:0"
}
```

**Response:**
```json
{
  "status": "ok"
}
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
