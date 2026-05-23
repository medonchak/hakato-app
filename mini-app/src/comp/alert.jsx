// AlertRuleForm.jsx
import React, { useState } from "react";
import { Box, Typography, TextField, Select, MenuItem, FormControl, InputLabel, FormControlLabel, Checkbox, Button, Stack, Chip, IconButton } from "@mui/material";
import DeleteIcon from "@mui/icons-material/Close";
import { warmStyles as S } from "../dashboardWarmStyles";
import { sendAlerts } from "../api"; // або використайте axios.post("/alerts", { payload })

export default function AlertForm(props) {
  const isEthAddress = (v) =>
    /^0x[a-fA-F0-9]{40}$/.test(v);

  const isPositiveNumber = (v) =>
    v !== "" && !isNaN(v) && Number(v) > 0;

  // Базові поля
  const [address, setAddress] = useState("");
  const [errors, setErrors] = useState({});

  // NFT
  const [nftMint, setNftMint] = useState(false);
  const [nftBuyEnabled, setNftBuyEnabled] = useState(false);
  const [nftBuyMin, setNftBuyMin] = useState("");        // число рядком
  const [nftBuyCurr, setNftBuyCurr] = useState("ETH");   // ETH | USDT

  const [nftSellEnabled, setNftSellEnabled] = useState(false);
  const [nftSellMin, setNftSellMin] = useState("");
  const [nftSellCurr, setNftSellCurr] = useState("ETH");

  // Swap
  const [swapEnabled, setSwapEnabled] = useState(false);
  const [swapMin, setSwapMin] = useState("");
  const [swapCurr, setSwapCurr] = useState("USDT");
  const [alertChain, setAlertChain] = useState("ETH");
  const [swapTokens, setSwapTokens] = useState([]);      // масив 0x...
  const [swapTokenInput, setSwapTokenInput] = useState("");

  // ETH transfer
  const [ethEnabled, setEthEnabled] = useState(false);
  const [ethMin, setEthMin] = useState("");  
  const [ethMode, setEthMode] = useState("BUY_ANY_BOTH"); 
  const [financeEnabled, setFinanceEnabled] = useState(false);
  const [minUSD, setMinUSD] = useState("");
  const [sellNative, setSellNative] = useState(false);
  const [buyNative, setBuyNative] = useState(false);
  const [buyAnyNative, setBuyAnyNative] = useState(false);
  const [buyAnyStable, setBuyAnyStable] = useState(false);
  const [ethCurrency, setEthCurrency] = useState("USD"); 
// USD | NATIVE
  // Аномалії (газ)
  const [anomalyEnabled, setAnomalyEnabled] = useState(false);
  const [anomalyGasGwei, setAnomalyGasGwei] = useState(""); // поріг gas price у Gwei
  const telegram = window.Telegram?.WebApp;
  const telegramId = telegram?.initDataUnsafe?.user?.id || 574595536;
  const disabledOthers = anomalyEnabled; // якщо аномалії увімкнено — решта не працює
 
  const addSwapToken = () => {
    const v = swapTokenInput.trim();
    if (!v) return;
    const low = v.toLowerCase();
    if (!swapTokens.includes(low)) setSwapTokens([...swapTokens, low]);
    setSwapTokenInput("");
  };
  const removeSwapToken = (t) => setSwapTokens(swapTokens.filter(x => x !== t));
  const validateForm = () => {
    const e = {};

    // ===== ADDRESS (обовʼязково) =====
    if (!address) {
      e.address = "Address is required";
    } else if (!isEthAddress(address)) {
      e.address = "Invalid address format";
    }

    // ===== CHECK: AT LEAST ONE CONDITION ENABLED =====
    const anyConditionEnabled =
      swapEnabled ||
      financeEnabled ||
      anomalyEnabled ||
      nftMint ||
      nftBuyEnabled ||
      nftSellEnabled;

    if (!anyConditionEnabled) {
      e.form = "You must enable at least one alert condition";
    }

    // ===== ANOMALY =====
    if (anomalyEnabled) {
      if (!anomalyGasGwei || !isPositiveNumber(anomalyGasGwei)) {
        e.anomalyGasGwei = "Gas price must be a positive number";
      }
    }

    // ===== SWAP =====
    if (swapEnabled) {
      if (!isPositiveNumber(swapMin)) {
        e.swapMin = "Minimum amount must be greater than 0";
      }

      if (!swapTokens || swapTokens.length === 0) {
        e.swapTokens = "At least one token contract is required";
      } else {
        for (const t of swapTokens) {
          if (!isEthAddress(t)) {
            e.swapTokens = "Invalid token contract address";
            break;
          }
        }
      }
    }

    // ===== FINANCE (NATIVE / STABLE) =====
    if (financeEnabled) {
      if (!minUSD || !isPositiveNumber(minUSD)) {
        e.minUSD = "Minimum USD amount is required";
      }

      if (
        !sellNative &&
        !buyNative &&
        !buyAnyNative &&
        !buyAnyStable
      ) {
        e.finance =
          "Select at least one financial operation type";
      }
    }

    // ===== NFT =====
    if (nftBuyEnabled) {
      if (!nftBuyMin || !isPositiveNumber(nftBuyMin)) {
        e.nftBuyMin = "Minimum buy amount must be greater than 0";
      }
    }

    if (nftSellEnabled) {
      if (!nftSellMin || !isPositiveNumber(nftSellMin)) {
        e.nftSellMin = "Minimum sell amount must be greater than 0";
      }
    }

    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const onSubmit = async (e) => {
   
    e.preventDefault();
    if (!validateForm()) return;
    
    const payload = {
      creator: String(telegramId),
      ruleId: props.ruleId,
      address,
      alertChain,
       // 0x...
      // якщо увімкнуто "Аномальні" — надсилаємо тільки їх
      anomalies: anomalyEnabled ? {
        enabled: true,
        minGasPriceGwei: anomalyGasGwei || ""
      } : null,

      // нижче — умови, які враховуються лише коли anomalies не увімкнено
      nft: !disabledOthers ? {
        mint: !!nftMint,
        buy: {
          enabled: !!nftBuyEnabled,
          minAmount: nftBuyMin || "",
          currency: nftBuyCurr // "ETH" | "USDT"
        },
        sell: {
          enabled: !!nftSellEnabled,
          minAmount: nftSellMin || "",
          currency: nftSellCurr
        }
      } : null,

      swap: !disabledOthers ? {
        enabled: !!swapEnabled,
        minAmount: swapMin || "",
        currency: swapCurr, // "ETH" | "USDT"
        tokens: swapTokens // список контрактів токенів 0x...
      } : null,

       swapFinance: financeEnabled  ? {
        enabled: true,
        minUsd: minUSD,
        allow: {
            sellNative: sellNative,       // true / false
            buyNative: buyNative,
            buyAnyNative: buyAnyNative,
            buyAnyStable: buyAnyStable,
          },     // BUY_NATIVE | SELL_NATIVE | BUY_ANY_NATIVE | BUY_ANY_STABLE | BUY_ANY_BOTH
       } : null,
    };

    // Надсилання
  
    try {
      const res = await sendAlerts(payload);
      if (res?.ok === false) {
        console.error("sendAlerts error", res?.error || res);
        return;
      }
      props.setmodalAlertForm(false);
    } catch (err) {
      console.error("sendAlerts error", err);
    }
    // опційно: очистити/показати повідомлення
  };

  return (
    <Box component="form" onSubmit={onSubmit} sx={{ p: 2, background: S.card, borderRadius: 2, boxShadow: '0 1.5px 8px #47a6c714', border: S.cardBorder }}>
      <Typography variant="h6" sx={{ color: S.accentBlue, fontWeight: 700, mb: 1.5 }}>
        Alert rule
      </Typography>

      {/* Адреса */}
      <TextField label="Address (0x...)"
        fullWidth
        size="small"
        sx={{ mb: 2}}
        error={!!errors.address}
        helperText={errors.address}
        value={address}
        onChange={(e)=>setAddress(e.target.value)}
      />
      {/* Мережа */}
      <FormControl size="small" sx={{ mb: 2, minWidth: 110 }}>
            <InputLabel sx={S.inputLabel}>Network</InputLabel>
            <Select value={alertChain} label="Currency" onChange={(e)=>setAlertChain(e.target.value)}  sx={S.select}>
              <MenuItem value="ETH">Ethereum</MenuItem>
              <MenuItem value="BSC">BNB Chain</MenuItem>
            </Select>
      </FormControl>

      {/* Swap */}
      <Box sx={{ opacity: disabledOthers ? 0.5 : 1, mb: 2 }}>
        <FormControlLabel
          control={<Checkbox checked={swapEnabled} onChange={(e)=>setSwapEnabled(e.target.checked)} disabled={disabledOthers} />}
          label="Token swap"
        />
        <Box sx={{ display:'flex', gap:1.5, flexWrap:'wrap', mt:1 }}>
          <TextField
            label="Minimum amount"
            size="small"
            disabled={!swapEnabled}
            value={swapMin}
            error={!!errors.swapMin}
            helperText={errors.swapMin}
            onChange={(e) => {
              const v = e.target.value;
              if (/^\d*\.?\d*$/.test(v)) {
                setSwapMin(v);
              }
            }}
          />
          {errors.swapTokens && (
            <Typography variant="caption" color="error">
              {errors.swapTokens}
            </Typography>
          )}
          <FormControl size="small" sx={{ minWidth: 110 }}>
            <InputLabel sx={S.inputLabel}>Currency</InputLabel>
            <Select value={swapCurr} label="Currency" onChange={(e)=>setSwapCurr(e.target.value)} disabled={!swapEnabled || disabledOthers} sx={S.select}>
              {alertChain=='ETH' ? <MenuItem value="ETH">ETH</MenuItem> :
              <MenuItem value="BNB">BNB</MenuItem>}
              <MenuItem value="USDT">USDT</MenuItem>
            </Select>
          </FormControl>
        </Box>
        <Box sx={{ mt: 1.5 }}>
          <Typography variant="body2" sx={{ mb: 0.5, color: S.gray }}>Token contracts (optional)</Typography>
          <Box sx={{ display:'flex', gap:1, mb:1 }}>
            <TextField
              label="0x…"
              size="small"
              disabled={!swapEnabled || disabledOthers}
              value={swapTokenInput}
              onChange={(e) => {
                const v = e.target.value.trim();
                if (/^(0x)?[0-9a-fA-F]*$/.test(v)) {
                  setSwapTokenInput(v);
                }
              }}
            />
            <Button variant="outlined" onClick={addSwapToken} disabled={!swapEnabled || disabledOthers}>Add</Button>
          </Box>
          <Stack direction="row" spacing={1} sx={{ flexWrap:'wrap' }}>
            {swapTokens.map(t => (
              <Chip key={t} label={t} onDelete={()=>removeSwapToken(t)} deleteIcon={<IconButton size="small"><DeleteIcon fontSize="small"/></IconButton>} />
            ))}
          </Stack>
        </Box>
      </Box>

      {/* FINANCE TRANSFERS (NATIVE / STABLE) */}
      {(alertChain === "ETH" || alertChain === "BSC") && (
      <Box sx={{ mb: 2 }}>
        <FormControlLabel
          control={
            <Checkbox
              checked={financeEnabled}
              onChange={(e) => setFinanceEnabled(e.target.checked)}
            />
          }
          label="Financial operations "
        />

        <Box
          sx={{
            mt: 1.5,
            p: 1.5,
            borderRadius: 2,
            background: "rgba(235,239,242,0.6)",
            opacity: financeEnabled ? 1 : 0.5,
            pointerEvents: financeEnabled ? "auto" : "none",
            transition: "opacity .25s ease",
          }}
        >
          {/* Типи операцій */}
          <Typography sx={{ fontSize: 13, fontWeight: 600, mb: 1 }}>
            Operation type
          </Typography>

          <Stack spacing={0.5}>
            <FormControlLabel
              control={
                <Checkbox
                  checked={sellNative}
                  onChange={(e) => setSellNative(e.target.checked)}
                />
              }
              label={alertChain == "ETH" ? "Sell ETH" : "Sell BNB"}
            />

            <FormControlLabel
              control={
                <Checkbox
                  checked={buyNative}
                  onChange={(e) => setBuyNative(e.target.checked)}
                />
              }
              label={alertChain == "ETH" ? "Buy ETH" : "Buy BNB"}
            />

            <FormControlLabel
              control={
                <Checkbox
                  checked={buyAnyNative}
                  onChange={(e) => setBuyAnyNative(e.target.checked)}
                />
              }
              label={alertChain == "ETH" ? "Buy tokens for ETH" : "Buy tokens for BNB"} 
            />

            <FormControlLabel
              control={
                <Checkbox
                  checked={buyAnyStable}
                  onChange={(e) => setBuyAnyStable(e.target.checked)}
                />
              }
              label="Buy tokens for stablecoins"
            />
          </Stack>

          {/* Мінімальна сума */}
          <TextField
            label="Minimum amount (USD)"
            size="small"
            sx={{ mt: 1.5 }}
            value={minUSD}
            error={!!errors.minUSD}
            helperText={errors.minUSD}
            onChange={(e) => {
              const v = e.target.value;
              if (/^\d*\.?\d*$/.test(v)) {
                setMinUSD(v);
              }
            }}
          />
        </Box>
      </Box>
      )}

      <Button type="submit" variant="contained" sx={{ fontWeight: 700, background: 'linear-gradient(90deg,#08f7fe,#fe53bb)' }}>
        Save rule
      </Button>
    </Box>
  );
}
