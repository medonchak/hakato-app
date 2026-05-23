# Hakato — On-Chain AI Trading Agent on Mantle

> Autonomous DeFi agent that detects token anomalies, optimises trading strategies with VWAP + Sharpe ratio, and executes every decision **verifiably on-chain** via Mantle smart contracts.

[![Mantle](https://img.shields.io/badge/Network-Mantle%205000-06b6d4?style=flat-square)](https://mantle.xyz)
[![Go](https://img.shields.io/badge/Backend-Go%201.24-00ADD8?style=flat-square)](https://go.dev)
[![React](https://img.shields.io/badge/Frontend-React%2019-61DAFB?style=flat-square)](https://react.dev)
[![Solidity](https://img.shields.io/badge/Contracts-Solidity%200.8.20-363636?style=flat-square)](https://soliditylang.org)
[![PWA](https://img.shields.io/badge/PWA-Enabled-5A0FC8?style=flat-square)](https://web.dev/progressive-web-apps)

---

## Problem

DeFi markets move fast. Retail traders miss anomalies, can't back-test strategies in real time, and have no way to **verify** that an agent actually followed its own rules — not just claimed to.

## Solution

Hakato is a full-stack AI agent that:

1. **Detects** unusual token activity on Mantle in real time (volume spikes, tx-count spikes, wallet concentration)
2. **Optimises** VWAP-based trading parameters for each token via grid search (Sharpe ratio)
3. **Executes** trades through a secure on-chain wallet (`AgentWallet.sol`) on Merchant Moe
4. **Logs every decision** to `SignalRegistry.sol` and every anomaly to `AnomalyLogger.sol` — fully verifiable on Mantle Explorer
5. **Notifies** users via Telegram bot and a PWA / Mini App

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      Mantle RPC (5000)                  │
│   Blocks → token_transfer_events → hourly_activity      │
└───────────────────┬─────────────────────────────────────┘
                    │ Go backend (scanner/)
          ┌─────────▼──────────┐
          │  Anomaly Detection │ ← token_anomaly_rules
          │  (real-time, per   │   (spike × volume/txcount,
          │   block)           │    dominance, concentration)
          └─────────┬──────────┘
                    │ anomaly found
          ┌─────────▼──────────┐        ┌──────────────────────┐
          │  AnomalyLogger.sol │        │  Strategy Optimizer  │
          │  (on-chain log)    │        │  VWAP grid search    │
          └────────────────────┘        │  Sharpe ratio rank   │
                                        │  runs every 6 hours  │
          ┌─────────────────────────────▼──────────────────────┐
          │              Agent Executor (Go)                   │
          │  reads best strategy → computes VWAP deviation     │
          │  price < VWAP×buyThreshold  → BUY signal           │
          │  price > VWAP×sellThreshold → SELL signal          │
          └─────────┬──────────────────────────────────────────┘
                    │
          ┌─────────▼──────────┐        ┌──────────────────────┐
          │ SignalRegistry.sol │        │   AgentWallet.sol    │
          │ (every signal      │───────►│   swap() via         │
          │  recorded before   │        │   Merchant Moe DEX   │
          │  execution)        │        │   maxTrade $5        │
          └────────────────────┘        │   dailyLimit $50     │
                                        │   cooldown 5 min     │
                                        └──────────────────────┘
                    │
          ┌─────────▼──────────────────────────────────────────┐
          │         Notifications                              │
          │   Telegram Bot  +  React PWA  +  Mini App         │
          └────────────────────────────────────────────────────┘
```

---

## Smart Contracts (Mantle Mainnet)

| Contract | Address | Purpose |
|----------|---------|---------|
| `AnomalyLogger.sol` | _deploy → update here_ | Logs every detected anomaly on-chain |
| `SignalRegistry.sol` | _deploy → update here_ | Records every BUY/SELL/HOLD signal before execution |
| `AgentWallet.sol` | _deploy → update here_ | Executes swaps via Merchant Moe with safety limits |

### AnomalyLogger.sol

```solidity
function logAnomaly(
    uint64  chainId,
    address token,
    string  calldata reason,   // "spike_volume; dominance"
    uint32  severity,          // scaled ×100
    uint64  hourTs,
    string  calldata txHash
) external onlyOwner returns (uint256 id)
```

### SignalRegistry.sol

```solidity
function recordSignal(
    uint64 chainId, address token, string tokenSymbol,
    SignalType signalType,   // BUY=1, SELL=2, HOLD=3
    string reason,
    uint32 confidence,       // basis points (7500 = 75%)
    uint32 vwapPeriod,
    int32  buyThresholdBps,
    int32  sellThresholdBps,
    uint256 priceUsd         // scaled ×1e8
) external onlyOwner returns (uint256 id)
```

### AgentWallet.sol

```solidity
function swap(
    address tokenIn, address tokenOut, uint24 fee,
    uint256 amountIn, uint256 amountOutMin,
    uint256 tradeUSD    // for safety limit check
) external onlyAgent returns (uint256 amountOut)
```

Safety defaults: `maxTradeUSD = $5`, `dailyLimitUSD = $50`, `cooldownSec = 300`

---

## Strategy Optimizer

Every 6 hours the optimizer runs a backtest over the last 30 days of hourly data for each Mantle token (MNT, mETH, USDY):

```
Grid search over:
  VWAP periods:      [4, 6, 8, 12, 14, 16, 24] hours
  Buy thresholds:    [-0.5% … -3.5%] below VWAP
  Sell thresholds:   [+0.5% … +4.0%] above VWAP
  Cooldown:          [1, 2, 4] hours

Ranking metric: Sharpe ratio (annualised from trade returns)
Minimum trades: 3 (to avoid overfitting)
```

Example output:
```
[optimizer] mETH: VWAP=14h buy=-2.8% sell=+3.1% Sharpe=1.84 WinRate=68% trades=22
```

The winning parameters are written to `SignalRegistry.sol` as the **active strategy** for that token.

---

## Anomaly Detection

Real-time per-block detection with 5 signal types:

| Signal | Description |
|--------|-------------|
| `spike_volume` | Hourly volume ≥ N× 7-day average |
| `spike_txcount` | Hourly tx count ≥ N× 24h average |
| `dominance` | Single transfer ≥ X% of hourly total |
| `concentration` | Top-1 address share ≥ threshold |
| `supply_pct` | Transfer ≥ X% of total token supply |

Severity is a composite score (0–1.5), written to `AnomalyLogger.sol`.

---

## Tracked Tokens on Mantle

| Token | Contract | Role |
|-------|----------|------|
| MNT | native | Native gas + trading asset |
| mETH | `0xcDA86A272531e8640cD7F1a92c01839911B90bb0` | Liquid staked ETH on Mantle |
| USDY | `0x5bE26527e817998A7206475496fDE1E68957c5A6` | Yield-bearing stablecoin |
| USDC | `0x09Bc4E0D864854c6aFB6eB9A9cdF58aC190D0dF9` | Stablecoin |
| USDT | `0x201EBa5CC46D216Ce6DC03F6a759e8E766e956aE` | Stablecoin |

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Blockchain | Mantle Network (chainId 5000) |
| Smart Contracts | Solidity 0.8.20 |
| DEX | Merchant Moe (Mantle) |
| Backend | Go 1.24 · Gorilla Mux · go-ethereum v1.16 |
| Database | MySQL 8 |
| Frontend | React 19 · TailwindCSS · Recharts · Framer Motion |
| Wallet | MetaMask (window.ethereum · EIP-1193) |
| Notifications | Telegram Bot API (polling) |
| PWA | service-worker.js · Web App Manifest |
| Mini App | Telegram Web App JS SDK |

---

## Project Structure

```
hakato-app/
├── contracts/
│   ├── AnomalyLogger.sol     # on-chain anomaly audit log
│   ├── SignalRegistry.sol    # on-chain signal + strategy registry
│   └── AgentWallet.sol       # agent-controlled DEX wallet
│
├── scanner/                  # Go backend
│   ├── main.go               # server entry, routes, chain clients
│   ├── agent_executor.go     # signal generation + on-chain execution
│   ├── strategy_optimizer.go # VWAP grid search, Sharpe ranking
│   ├── token_anomaly_runtime.go  # real-time anomaly evaluation
│   ├── token_analytics_workers.go # hourly/daily materialization
│   ├── telegram.go           # bot: /agent_status /best_strategy
│   ├── database.go           # MySQL schema + CRUD
│   └── ...
│
├── mini-app/                 # React PWA + Telegram Mini App
│   ├── src/
│   │   ├── comp/
│   │   │   ├── AgentDashboard.jsx  # VWAP chart · trades · strategy
│   │   │   └── WalletPanel.jsx     # MetaMask connect · balances · trade amount
│   │   ├── hooks/
│   │   │   └── useWallet.js        # EIP-1193 hook · Mantle network switch
│   │   └── ...
│   └── public/
│       ├── manifest.json     # PWA manifest
│       └── service-worker.js # offline cache
│
└── mysql/                    # DB schema dumps (28 tables)
```

---

## Getting Started

### Prerequisites

- Go 1.24+
- Node.js 18+
- MySQL 8
- MetaMask browser extension

### 1. Clone

```bash
git clone https://github.com/medonchak/hakato-app.git
cd hakato-app
```

### 2. Backend

```bash
cd scanner
cp .env.example .env   # fill in values (see Environment Variables)
go build -o hakato .
./hakato
```

Server starts on `http://localhost:8080`

### 3. Frontend

```bash
cd mini-app
npm install
npm start              # dev server on http://localhost:3000
```

### 4. Deploy Contracts

```bash
# Using Hardhat or Foundry on Mantle mainnet
# Set PRIVATE_KEY and MANTLE_RPC in your deploy config

forge create contracts/AnomalyLogger.sol:AnomalyLogger \
  --rpc-url https://rpc.mantle.xyz --private-key $PRIVATE_KEY

forge create contracts/SignalRegistry.sol:SignalRegistry \
  --rpc-url https://rpc.mantle.xyz --private-key $PRIVATE_KEY

forge create contracts/AgentWallet.sol:AgentWallet \
  --constructor-args $MERCHANT_MOE_ROUTER $AGENT_KEY \
  --rpc-url https://rpc.mantle.xyz --private-key $PRIVATE_KEY
```

Then update `.env` with the deployed addresses.

---

## Environment Variables

Create `scanner/.env`:

```env
# Telegram Bot (from @BotFather)
TG_BOT_TOKEN=your_bot_token
TG_ADMIN_CHAT_ID=your_telegram_id

# On-chain Agent
AGENT_ENABLED=true
AGENT_PRIVATE_KEY=0x...          # agent wallet private key
SIGNAL_REGISTRY_ADDR=0x...       # deployed SignalRegistry.sol
ANOMALY_LOGGER_ADDR=0x...        # deployed AnomalyLogger.sol
AGENT_WALLET_ADDR=0x...          # deployed AgentWallet.sol
```

---

## API Reference

### Agent

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/agent/signals?token=mETH&chain_id=5000` | Last 100 signals for token |
| `GET` | `/api/agent/strategy?token=mETH&chain_id=5000` | Best strategy params |
| `GET` | `/api/agent/position?chain_id=5000` | Current position + PnL |
| `GET/POST` | `/api/agent/trade-config` | Get / set trade amount |

### Market & Tokens

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/market/top-active` | Top active tokens |
| `GET` | `/api/token/hourly?token=&chain_id=` | Hourly VWAP metrics |
| `GET` | `/api/token/anomalies?token=&chain_id=` | Recent anomaly events |
| `GET` | `/api/token/dashboard?token=&chain_id=` | Full token dashboard |

### Portfolios

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/portfolios/create` | Create portfolio |
| `GET` | `/api/portfolios?user_id=` | List portfolios |
| `POST` | `/api/portfolio/operation` | Record buy/sell |

---

## Telegram Bot Commands

| Command | Description |
|---------|-------------|
| `/start` | Welcome + command list |
| `/status` | Backend health check |
| `/agent_status` | Current positions and PnL |
| `/best_strategy mETH` | Optimal VWAP params for token |

---

## Database Schema (key tables)

```sql
-- Hourly on-chain activity (base for anomaly detection + VWAP)
token_hourly_activity (chain_id, token, hour_ts, transfer_count,
                       total_volume_usd, exchange_in_usd, exchange_out_usd)

-- Agent decisions
agent_signals  (chain_id, token, signal_type, reason, confidence,
                price_usd, vwap, tx_hash, on_chain_id, created_at)

-- Optimizer results
agent_strategies (chain_id, token, vwap_period, buy_threshold_pct,
                  sell_threshold_pct, sharpe, win_rate, total_trades)

-- Anomaly events
token_anomaly_events (chain_id, token, severity, reason, tx_hash, created_at)
```

---

## Key User Flows

### Flow 1 — Anomaly → On-chain Log
```
Block arrives → token transfer detected → hourly metrics updated
→ EvaluateSystemAnomaly() triggers → AnomalyLogger.sol.logAnomaly()
→ Telegram notification → visible in Agent tab
```

### Flow 2 — Optimizer → On-chain Strategy
```
Every 6h: loadHourlyPoints(30d) → backtest grid search
→ best Sharpe params found → saved to agent_strategies DB
→ SignalRegistry.sol.updateStrategy() called
```

### Flow 3 — Signal → Trade
```
Agent tick (15 min) → load best strategy → compute VWAP
→ deviation crosses threshold → BUY/SELL signal
→ SignalRegistry.sol.recordSignal() → AgentWallet.sol.swap()
→ Telegram + PWA notification
```

### Flow 4 — Wallet Connect
```
User taps "Підключити MetaMask" → wallet_addEthereumChain (Mantle 5000)
→ eth_requestAccounts → read MNT/mETH/USDY/USDC/USDT balances
→ select trade token → input amount (or 25/50/75/100%)
→ POST /api/agent/trade-config → agent uses this amount for next trades
```

---

## Hackathon Tracks

- **Track 01** — DeFi / Trading Infrastructure on Mantle
- **Track 02** — AI Agents on Mantle

---

## Roadmap

- [x] Mantle Network integration (chainId 5000)
- [x] Real-time anomaly detection (5 signal types)
- [x] VWAP strategy optimizer (Sharpe ratio grid search)
- [x] AnomalyLogger.sol — on-chain anomaly audit
- [x] SignalRegistry.sol — on-chain signal registry
- [x] AgentWallet.sol — autonomous DEX execution
- [x] MetaMask wallet connect + balance display
- [x] Trade amount selector (% presets)
- [x] Telegram bot notifications
- [x] PWA support (offline cache + installable)
- [x] Telegram Mini App
- [ ] Live contract deployment on Mantle mainnet
- [ ] Multi-agent parallel strategies
- [ ] Cross-chain bridge integration

---

## License

MIT
