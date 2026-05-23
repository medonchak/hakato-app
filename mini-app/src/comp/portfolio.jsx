import React, { useState, useEffect } from "react";
import { sendToken, getPortfolioTokens, addTokenToPortfolio, portfolioOperation } from "../api";
import { Button, IconButton, Skeleton, Modal, Box, FormControl, InputLabel } from "@mui/material";
import DeleteIcon from "@mui/icons-material/Delete";
import { warmStyles as S } from "../dashboardWarmStyles";
import TokenAnomalyToggle from "./TokenAnomalyToggle";
import Icon from "./Icon";

import TokenDashboard from "./TokenDashboard";
import CancelIcon from "@mui/icons-material/Cancel";
const CryptoPortfolio = ({visioDash,setvisioDash, portfolioName, portfolioId,onDeletePortfolioCard, closeModal,anomaly_alerts_enabled }) => {
  /* ===========================================================
     СТЕЙТИ
     =========================================================== */
  const [showAddToken, setShowAddToken] = useState(false);
  const [showAddTokenModal, setShowAddTokenModal] = useState(false);
  const [addTokenStep, setAddTokenStep] = useState(null); // null | 'select' | 'contract'
  const [tokens, setTokens] = useState([]);
  const [prices, setPrices] = useState({});
  const [loadData, setloadData] = useState(false);
  const [tokenDataModal, settokenDataModal] = useState({});

  const [loadingContracts, setLoadingContracts] = useState(new Set());
  const [newToken, setNewToken] = useState({
        chain: "ETH",
        contract: "",
        amount: "",
        invested: "",
  });
  const [plMode, setPlMode] = useState({});
  const [fixModal, setFixModal] = useState(null);

  /* ===========================================================
     PRESET TOKENS CONFIGURATION
     =========================================================== */
  const PRESET_TOKENS = {
    ETH: [
      { label: "ETH", contract: "native", symbol: "ETH" },
      { label: "USDT", contract: "0xdAC17F958D2ee523a2206206994597C13D831ec7", symbol: "USDT" },
      { label: "USDC", contract: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606e48", symbol: "USDC" },
      { label: "DAI", contract: "0x6B175474E89094C44Da98b954EedeAC495271d0F", symbol: "DAI" },
      { label: "BTC", contract: "BTC", symbol: "BTC" }, // логічний BTC
    ],
    BSC: [
      { label: "BNB", contract: "native", symbol: "BNB" },
      { label: "USDT", contract: "0x55d398326f99059fF775485246999027B3197955", symbol: "USDT" },
      { label: "USDC", contract: "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d", symbol: "USDC" },
      { label: "BUSD", contract: "0xe9e7cea3dedca5984780bafc599bd69add087d56", symbol: "BUSD" },
      { label: "BTC", contract: "BTC", symbol: "BTC" },
    ],
  };

  /* ===========================================================
     ФОРМАТЕРИ
     =========================================================== */
  const fmt = (n, d = 2) =>
    n == null || isNaN(n) ? "—" : Number(n).toFixed(d);

  const fmtQty = (n) =>
    n == null
      ? "—"
      : Number(n).toFixed(6).replace(/0+$/, "").replace(/\.$/, "");

  const fmtPrice = (n) => {
    if (!n || isNaN(n)) return "—";
    const s = n < 0.01 ? n.toFixed(6) : n.toFixed(4);
    return "$" + s.replace(/0+$/, "").replace(/\.$/, "");
  };

  /* ===========================================================
     1) ЗАВАНТАЖЕННЯ ТОКЕНІВ
     =========================================================== */
  const loadPortfolioTokens = async () => {
    try {
      const res = await getPortfolioTokens(portfolioId);
      if (Array.isArray(res)) {
       
          const resmap = res.map((t) => ({
            id: t.ID,
            amount: Number(t.Amount),
            invested: Number(t.Invested_usd ?? t.Invested ?? 0),
            realized: Number(t.Realized ?? 0),
            symbol: t.Symbol,
            buyprice: t.BuyPriceUSD,
            price: t.CurrentPriceUSD,
          }))
      
           setTokens(resmap);
           setloadData(true)
      }
    } catch (e) {
      console.error("Load tokens error:", e);
    }
  };

  useEffect(() => {
    if (portfolioId) 
    loadPortfolioTokens();

    const interval = setInterval(() => {
    loadPortfolioTokens(); // оновлення
  }, 5_000); // 10–30 сек — нормально

  return () => clearInterval(interval);
  }, [portfolioId]);
  
  /* ===========================================================
     2) ДОДАВАННЯ ТОКЕНА
     =========================================================== */
  const addToken = async () => {
    const { chain, contract, amount, invested, } = newToken;
    if (!contract?.trim() || !amount || !invested) return;

    const body = {
      portfolioId,
      chain,
      contract: contract.trim().toLowerCase(),
      amount: Number(amount),
      invested: Number(invested),
    };

    try {
      const res = await addTokenToPortfolio(body);
   
      if (res?.ok) await loadPortfolioTokens();
      setNewToken({
        chain: "ETH",
        contract: "",
        amount: "",
        invested: "",
      })
    } catch (e) {
      console.error("Add token error:", e);
    }

  };

  /* ===========================================================
     3) ОТРИМАННЯ ЦІН
     =========================================================== */
  // const fetchPrices = async () => {
  //   const contracts = [...new Set(tokens.map((t) => t.contract))];
  //   if (!contracts.length) return;

  //   try {
  //     const req = {};
  //     contracts.forEach((c, i) => (req[i] = c));
  //     const res = await sendToken(req);

  //     const map = {};
  //     if (Array.isArray(res)) {
  //       res.forEach((i) => {
  //         if (i?.address) {
  //           map[i.address.toLowerCase()] = {
  //             price: +i.price_usd || 0,
  //             symbol: i.symbol?.toUpperCase() || "???",
  //           };
  //         }
  //       });
  //     }

  //     setPrices(map);
  //     setLoadingContracts(new Set());
  //     setloadData(true)
  //   } catch (e) {
  //     console.error("Prices error:", e);
  //   }
  // };

  // useEffect(() => {
  //   fetchPrices();
  //   const interval = setInterval(fetchPrices, 20000);
  //   return () => clearInterval(interval);
  // }, [tokens]);

  /* ===========================================================
     4) ЛОГІКА P/L
     =========================================================== */
    const getTokenData = (t) => {
      const price = Number(t.price || 0); // ⬅️ БЕРЕМО З СЕРВЕРА

      const value = price * t.amount;
      const unrealized = value - t.invested;
      const totalPL = unrealized + (t.realized || 0);
      const totalPLPct = t.invested > 0 ? (totalPL / t.invested) * 100 : 0;
      const realized = t.realized
      const avgEntry = t.amount > 0 ? t.invested / t.amount : 0;

      return {
        price,
        value,
        unrealized,
        totalPL,
        totalPLPct,
        avgEntry,
        realized,
      };
    };


  /* ===========================================================
     5) FIX-МЕХАНІКА
     =========================================================== */
 

  const confirmFix = async () => {
  if (!fixModal) return;

  const token = tokens.find(t => t.id === fixModal.tokenId);
  if (!token) return setFixModal(null);

  const price = Number(token.price) || 0;
  const amount = Number(token.amount) || 0;
  const invested = Number(token.invested) || 0;

  if (price <= 0 || amount <= 0) return setFixModal(null);

  const percent = Number(fixModal.percent || 0);
  if (percent <= 0 || percent > 100) return setFixModal(null);

  // ✅ ПОВНА ВАРТІСТЬ ПОЗИЦІЇ
  const totalValue = price * amount;

  // ✅ СКІЛЬКИ ФІКСУЄМО
  const usd = totalValue * (percent / 100);
  const amountDelta = -(amount * (percent / 100));

  let body;

  // 🅰️ У КЕШ
  if (!fixModal.targetTokenId) {
    body = {
      type: "REALIZE_CASH",
      portfolioId,
      from: {
        tokenId: token.id,
        amountDelta,
        realizedDelta: usd,
      },
    };
  }

  // 🅲 У НОВИЙ ТОКЕН
  else if (fixModal.targetTokenId === "__new__") {
    const contract = fixModal.newContract?.trim().toLowerCase();
    if (!contract) return;

    body = {
      type: "REALIZE_NEW_TOKEN",
      portfolioId,
      from: {
        tokenId: token.id,
        amountDelta,
        realizedDelta: usd,
      },
      newToken: {
        contract,
        invested: usd,
      },
    };
  }

  // 🅱️ У ІСНУЮЧИЙ ТОКЕН
  else {
    const target = tokens.find(t => t.id === fixModal.targetTokenId);
    if (!target || !target.price) return;

    body = {
      type: "REALIZE_SWAP",
      portfolioId,
      from: {
        tokenId: token.id,
        amountDelta,
        realizedDelta: usd,
      },
      to: {
        tokenId: target.id,
        amountDelta: usd / target.price,
        investedDelta: usd,
      },
    };
  }

  try {
    await portfolioOperation(body);
    await loadPortfolioTokens();
  } finally {
    setFixModal(null);
  }
    };

  /* ===========================================================
     6) ЗАГАЛЬНА СТАТИСТИКА
     =========================================================== */
  const totalValue = tokens.reduce((a, t) => a + getTokenData(t).value, 0);
  const totalInvested = tokens.reduce((a, t) => a + t.invested, 0);
  const totalRealized = tokens.reduce((a, t) => a + t.realized, 0);
  const totalPL = totalValue - totalInvested + totalRealized;

  /* ===========================================================
     СТИЛІ
     =========================================================== */
  const C = {
    bg: "#f6f3eb",
    card: "#ffffff",
    border: "#e5ddd0",
    soft: "#8b857c",
    accent: "#2f7eed",
    green: "#27a36a",
    red: "#d9534f",
    shadow: "0 4px 14px rgba(0,0,0,0.06)",
  };

  const inputStyle = {
    width: "100%",
    padding: "12px",
    borderRadius: "14px",
    border: `1px solid ${C.border}`,
    background: "#fbf8f3",
    fontSize: "14px",
    color: "#222",
    boxSizing: "border-box",
  };
 
  /* ===========================================================
     UI
     =========================================================== */
  return (
    <div
      style={{
        minHeight: "100vh",
        background: C.bg,
        padding: "16px 12px 120px",
        fontFamily: "Inter, -apple-system, system-ui, sans-serif",
        maxWidth: 480,
        margin: "0 auto",
      }}
    >
    

      {/* TITLE */}
      <h1
        style={{
          textAlign: "center",
          fontSize: "24px",
          fontWeight: 800,
          marginBottom: "18px",
        }}
      >
       Portfolio {portfolioName}

           <IconButton  onClick={() => {
              onDeletePortfolioCard(portfolioId)
              closeModal(false)
              }}>
                  <DeleteIcon  />
            </IconButton>
            <TokenAnomalyToggle
                portfolioId={portfolioId}
                enabled={anomaly_alerts_enabled}
              />

      </h1>

      {/* ======== СТАТИСТИКА ======== */}
      {Array.isArray(tokens) && tokens.length ? 
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "1fr 1fr",
          gap: "10px",
          marginBottom: "20px",
        }}
      >
        {/* Вартість */}
        <div
          style={{
            background: C.card,
            padding: "12px",
            borderRadius: "16px",
            border: `1px solid ${C.border}`,
            boxShadow: C.shadow,
          }}
        >
          <div style={{ color: C.soft, fontSize: "11px" }}>Value</div>
          <div style={{ fontSize: "18px", fontWeight: 700 }}>
             {loadData !== false && Array.isArray(tokens) && tokens.length > 0 ? (
              <>
                 ${fmt(totalValue)}
              </>
            ) : (
              <Skeleton />
            )}
           
          </div>
        </div>

        {/* Вкладено */}
        <div
          style={{
            background: C.card,
            padding: "12px",
            borderRadius: "16px",
            border: `1px solid ${C.border}`,
            boxShadow: C.shadow,
          }}
        >
          <div style={{ color: C.soft, fontSize: "11px" }}>Invest</div>
          <div style={{ fontSize: "18px", fontWeight: 700 }}>
            ${fmt(totalInvested)}
          </div>
        </div>

        {/* P/L */}
        <div
          style={{
            background: C.card,
            padding: "12px",
            borderRadius: "16px",
            border: `1px solid ${C.border}`,
            boxShadow: C.shadow,
            gridColumn: "span 2",
            textAlign: "center",
          }}
        >
          <div style={{ color: C.soft, fontSize: "11px" }}>P/L</div>
          <div
            style={{
              fontSize: "18px",
              fontWeight: 700,
              color: totalPL >= 0 ? C.green : C.red,
            }}
          >
          {loadData !== false > 0 ? (
              <>
                {totalPL >= 0 ? "+" : ""}
                {fmt(totalPL)}
              </>
            ) : (
              <Skeleton />
            )}
          </div>
        </div>
      </div> :<></> }

      {/* ADD TOKEN BUTTON */}
      {!showAddToken && (
      <div
          onClick={() => {
            setShowAddTokenModal(true);
            setAddTokenStep('select');
          }}
          style={{
            height: "28px",
            cursor: "pointer",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            borderTop: `1px solid ${C.border}`,
            color: C.muted,
            fontSize: "12px",
            fontWeight: 600,
            userSelect: "none",
          }}
        >
          {showAddToken ? "Hide new token ▲" : "Add new token ▼"}
        </div>
      )}


      {/* MODALS */}
      {/* ======== GRID WITH SCROLL ======== */}
      <div style={{
          maxHeight: "calc(100vh - 420px)",
          overflowY: "auto",
          paddingRight: "8px",
          marginRight: "-8px",
        }}
      >
      <div style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fill, minmax(160px, 1fr))",
          gap: "12px",
        }}
      >
      {tokens.map((t) => {
          const d = getTokenData(t);
          const mode = plMode[t.id] || "usd";
    
                   return (
            <div
              key={t.id}
              style={{
                background: C.card,
                padding: "12px",
                borderRadius: "18px",
                border: `1px solid ${C.border}`,
                boxShadow: C.shadow,
                position: "relative",
              }}
            >
     
     
                   {/* HEADER */}
              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  marginBottom: "8px",
                }}
              >
                <div 
                  style={{
                    fontSize: "17px",
                    fontWeight: 800,
                    color: C.accent,
                  }}
                >
                  {t.symbol}   
                  <Button onClick={()=>{ 
                    setvisioDash(true);
                    settokenDataModal(t)
                    }} style={{width:'0px'}}>
                    <Icon />
                    </Button> 
                </div>

                <div
                  onClick={() =>
                    setPlMode((p) => ({
                      ...p,
                      [t.id]: mode === "usd" ? "pct" : "usd",
                    }))
                  }
                  style={{
                    fontSize: "13px",
                    fontWeight: 700,
                    cursor: "pointer",
                    color: d.totalPL >= 0 ? C.green : C.red,
                  }}
                >
                  {d.totalPL >= 0 ? "+" : ""}
                  {mode === "usd"
                    ? `$${fmt(d.totalPL)}` 
                    : `${fmt(d.totalPLPct, 1)}%`}
                </div>
              </div>

              {/* ROWS */}
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns: "1fr 1fr",
                  gap: "6px",
                }}
              >
                <div>
                  <div style={{ fontSize: "11px", color: C.soft }}>Price:</div>
                  <strong style={{ fontSize: "13px" }}>
                    {fmtPrice(t.price)!=="—" ? fmtPrice(t.price) :  <Skeleton />}
                  </strong>
                </div>

                <div>
                  <div style={{ fontSize: "11px", color: C.soft }}>Amount:</div>
                  <strong style={{ fontSize: "13px" }}>
                    {fmtQty(t.amount.toFixed(2))}
                  </strong>
                </div>

                <div>
                  <div style={{ fontSize: "11px", color: C.soft }}>
                    Invest:
                  </div>
                  <strong style={{ fontSize: "13px" }}>
                    ${fmt(t.invested)}
                  </strong>
                </div>

                  <div>
                    <div style={{ fontSize: "11px", color: C.soft }}>
                        Price buy:
                    </div>
                    <strong style={{ fontSize: "13px" }}>
                      ${fmt(t.buyprice)}
                    </strong>
                  </div>

                <div>
                  <div style={{ fontSize: "11px", color: C.soft }}>
                    unrealized profit
                  </div>
                  <strong
                    style={{
                      fontSize: "13px",
                      color: d.unrealized >= 0 ? C.green : C.red,
                    }}
                  >
                    ${fmt(d.unrealized)}
                  </strong>
                </div>

                <div>
                  <div style={{ fontSize: "11px", color: C.soft }}>
                    realized profit
                  </div>
                  <strong style={{ fontSize: "13px", color: C.green }}>
                    ${fmt(t.realized)}
                  </strong>
                </div>
              </div>

              <button
                onClick={() =>
                  setFixModal({
                    tokenId: t.id,
                    usdAmount: "",
                    targetContract: "",
                  })
                }
                style={{
                  marginTop: "10px",
                  width: "100%",
                  padding: "9px",
                  borderRadius: "14px",
                  background: C.green,
                  color: "#fff",
                  border: "none",
                  fontSize: "13px",
                  fontWeight: 700,
                }}
              >
                realize profits
              </button>
             
           
            </div>
          );
      })}
      </div>
      </div>

      {/* FIX MODAL */}

    
      {fixModal && (() => {
        const token = tokens.find(t => t.id === fixModal.tokenId);
        if (!token) return null;

        const price = Number(token.price) || 0;
        const amount = Number(token.amount) || 0;
        const totalValue = price * amount;

        const percent = Number(fixModal.percent || 0);
        const usd = totalValue > 0 ? totalValue * (percent / 100) : 0;
        const qty = amount > 0 ? amount * (percent / 100) : 0;

        const isNewToken = fixModal.targetTokenId === "__new__";
        const portfolioTokens = tokens.filter(t => t.id !== token.id);
        const chain = newToken.chain || "ETH";
        const stables = PRESET_TOKENS[chain].filter(t => ['USDT', 'USDC', 'DAI', 'BUSD'].includes(t.symbol));
        const nativeTokens = PRESET_TOKENS[chain].filter(t => ['ETH', 'BNB'].includes(t.symbol));

        return (
          <div
            onClick={() => setFixModal(null)}
            style={overlayStyle}
          >
            <div
              onClick={(e) => e.stopPropagation()}
              style={{ ...modalStyle, maxHeight: '88vh' }}
            >
              {/* TITLE */}
              <div style={titleStyle}>Realize {token.symbol} Position</div>

              {/* PERCENT */}
              <label style={labelStyle}>Realization percentage (%)</label>
              <input
                type="number"
                inputMode="decimal"
                placeholder="0 – 100"
                min="0"
                max="100"
                value={fixModal.percent ?? ""}
                onChange={(e) => {
                  const val = Math.max(0, Math.min(100, Number(e.target.value) || 0));
                  setFixModal({ ...fixModal, percent: val });
                }}
                style={inputStyle}
              />

              {/* PREVIEW */}
              {percent > 0 && (
                <div style={previewStyle}>
                  <div style={{ marginBottom: 8 }}>
                    Total value: <b>${fmt(totalValue)}</b>
                  </div>
                  <div>
                    Will realize: <b>${fmt(usd)}</b> ({fmtQty(qty)} {token.symbol})
                  </div>
                </div>
              )}

              {/* TARGET OPTIONS */}
              <label style={{ ...labelStyle, marginTop: 12 }}>Realize to</label>

              {/* 💰 CASH */}
              <button
                onClick={() => setFixModal({ ...fixModal, targetTokenId: '' })}
                style={{
                  width: '100%',
                  padding: '10px',
                  borderRadius: 12,
                  border: fixModal.targetTokenId === '' ? '2px solid #2f7eed' : '1px solid #ddd',
                  background: fixModal.targetTokenId === '' ? '#e8f1ff' : '#f9f9f9',
                  cursor: 'pointer',
                  fontWeight: 600,
                  color: '#333',
                  fontSize: 13,
                  marginBottom: 8,
                  transition: 'all 0.2s'
                }}
              >
                💰 Cash
              </button>

              {/* 📊 PORTFOLIO TOKENS */}
              {portfolioTokens.length > 0 && (
                <>
                  <div style={{ fontSize: 11, fontWeight: 700, color: '#666', marginBottom: 6 }}>
                    Portfolio Tokens
                  </div>
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, marginBottom: 8 }}>
                    {portfolioTokens.map(t => (
                      <button
                        key={t.id}
                        onClick={() => setFixModal({ ...fixModal, targetTokenId: t.id })}
                        style={{
                          padding: '8px',
                          borderRadius: 10,
                          border: fixModal.targetTokenId === t.id ? '2px solid #2f7eed' : '1px solid #ddd',
                          background: fixModal.targetTokenId === t.id ? '#e8f1ff' : '#f9f9f9',
                          cursor: 'pointer',
                          fontWeight: 600,
                          color: '#333',
                          fontSize: 12,
                        }}
                      >
                        {t.symbol}
                      </button>
                    ))}
                  </div>
                </>
              )}

              {/* 💵 STABLES */}
              {stables.length > 0 && (
                <>
                  <div style={{ fontSize: 11, fontWeight: 700, color: '#666', marginBottom: 6 }}>
                    Stablecoins
                  </div>
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, marginBottom: 8 }}>
                    {stables.map(t => (
                      <button
                        key={t.contract}
                        onClick={() => setFixModal({ ...fixModal, targetTokenId: '__new__', newContract: t.contract })}
                        style={{
                          padding: '8px',
                          borderRadius: 10,
                          border: '1px solid #ddd',
                          background: '#f9f9f9',
                          cursor: 'pointer',
                          fontWeight: 600,
                          color: '#333',
                          fontSize: 12,
                        }}
                      >
                        {t.label}
                      </button>
                    ))}
                  </div>
                </>
              )}

              {/* 🔗 NATIVE TOKENS */}
              {nativeTokens.length > 0 && (
                <>
                  <div style={{ fontSize: 11, fontWeight: 700, color: '#666', marginBottom: 6 }}>
                    Native Tokens
                  </div>
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, marginBottom: 8 }}>
                    {nativeTokens.map(t => (
                      <button
                        key={t.contract}
                        onClick={() => setFixModal({ ...fixModal, targetTokenId: '__new__', newContract: t.contract })}
                        style={{
                          padding: '8px',
                          borderRadius: 10,
                          border: '1px solid #ddd',
                          background: '#f9f9f9',
                          cursor: 'pointer',
                          fontWeight: 600,
                          color: '#333',
                          fontSize: 12,
                        }}
                      >
                        {t.label}
                      </button>
                    ))}
                  </div>
                </>
              )}

              {/* CUSTOM CONTRACT */}
              <button
                onClick={() => setFixModal({ ...fixModal, targetTokenId: '__new__', newContract: '' })}
                style={{
                  width: '100%',
                  padding: '8px',
                  borderRadius: 10,
                  border: fixModal.targetTokenId === '__new__' && !fixModal.newContract ? '2px solid #2f7eed' : '1px solid #ddd',
                  background: '#f9f9f9',
                  cursor: 'pointer',
                  fontWeight: 600,
                  color: '#333',
                  fontSize: 12,
                  marginBottom: 8,
                }}
              >
                ➕ Custom Contract
              </button>

              {/* NEW TOKEN CONTRACT INPUT */}
              {isNewToken && (
                <>
                  <label style={labelStyle}>Token contract</label>
                  <input
                    type="text"
                    placeholder="0x..."
                    value={fixModal.newContract || ""}
                    onChange={(e) =>
                      setFixModal({ ...fixModal, newContract: e.target.value })
                    }
                    style={{ ...inputStyle, marginBottom: 12 }}
                  />
                </>
              )}

              {/* ACTIONS */}
              <div style={actionsStyle}>
                <button
                  onClick={confirmFix}
                  disabled={
                    percent <= 0 ||
                    percent > 100 ||
                    totalValue <= 0 ||
                    (isNewToken && !fixModal.newContract)
                  }
                  style={{
                    ...primaryBtnStyle,
                    opacity:
                      percent <= 0 ||
                      percent > 100 ||
                      totalValue <= 0 ||
                      (isNewToken && !fixModal.newContract)
                        ? 0.5
                        : 1,
                  }}
                >
                  Confirm Realization
                </button>

                <button
                  onClick={() => setFixModal(null)}
                  style={secondaryBtnStyle}
                >
                  Cancel
                </button>
              </div>
            </div>
          </div>
        );
      })()}

      <Modal
          open={visioDash}
          onClose={() => setvisioDash(false)}
        >
      <Box
        sx={{
          position: "fixed",
          top: "50%",
          left: "50%",
          transform: "translate(-50%, -50%)",
          width: "92%",
          maxWidth: 980,
          maxHeight: "88vh",
          overflowY: "auto",
          borderRadius: 4,
          bgcolor: "#F9F6F1",
          boxShadow: "0 30px 80px rgba(0,0,0,0.25)",
          p: { xs: 2, sm: 3 },
        }}
      >
              <Button   sx={{
          position: "absolute",
          top: 16,
          right: 16,
          zIndex: 10,
        }}>
                <CancelIcon onClick={() => setvisioDash(false)} />
              </Button>
              <TokenDashboard token={tokenDataModal}   />
      </Box>
    </Modal>

      {/* 🎯 ADD TOKEN SELECTOR MODAL */}
      {showAddTokenModal && addTokenStep === 'select' && (() => {
        const chain = newToken.chain || "ETH";
        const presetList = PRESET_TOKENS[chain] || [];
        const stables = presetList.filter(t => ['USDT', 'USDC', 'DAI', 'BUSD'].includes(t.symbol));
        const nativeTokens = presetList.filter(t => ['ETH', 'BNB'].includes(t.symbol));
        
        return (
          <div
            onClick={() => setShowAddTokenModal(false)}
            style={overlayStyle}
          >
            <div
              onClick={(e) => e.stopPropagation()}
              style={modalStyle}
            >
              <div style={titleStyle}>Add new token</div>

              {/* NETWORK SELECT */}
              <div style={{ marginBottom: 12 }}>
                <div style={{ fontSize: 12, fontWeight: 700, color: '#666', marginBottom: 6 }}>Network</div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
                  {['ETH', 'BSC'].map(net => (
                    <button
                      key={net}
                      onClick={() => setNewToken({ ...newToken, chain: net, preset: '', contract: '', symbol: '' })}
                      style={{
                        padding: '10px',
                        borderRadius: 12,
                        border: newToken.chain === net ? '2px solid #2f7eed' : '1px solid #ddd',
                        background: newToken.chain === net ? '#e8f1ff' : '#f9f9f9',
                        cursor: 'pointer',
                        fontWeight: 600,
                        color: '#333',
                        fontSize: 13,
                      }}
                    >
                      {net === 'ETH' ? 'Ethereum' : 'BSC'}
                    </button>
                  ))}
                </div>
              </div>

              {/* STABLECOINS */}
              {stables.length > 0 && (
                <>
                  <div style={{ fontSize: 12, fontWeight: 700, color: '#666', marginBottom: 6 }}>Stablecoins</div>
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, marginBottom: 12 }}>
                    {stables.map(t => (
                      <button
                        key={t.contract}
                        onClick={() => {
                          setNewToken({ ...newToken, preset: t.contract, contract: t.contract, symbol: t.symbol });
                          setAddTokenStep('amounts');
                        }}
                        style={{
                          padding: '10px',
                          borderRadius: 12,
                          border: '1px solid #ddd',
                          background: '#f9f9f9',
                          cursor: 'pointer',
                          fontWeight: 600,
                          color: '#333',
                          fontSize: 13,
                        }}
                      >
                        {t.label}
                      </button>
                    ))}
                  </div>
                </>
              )}

              {/* NATIVE TOKENS */}
              {nativeTokens.length > 0 && (
                <>
                  <div style={{ fontSize: 12, fontWeight: 700, color: '#666', marginBottom: 6 }}>Native Tokens</div>
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, marginBottom: 12 }}>
                    {nativeTokens.map(t => (
                      <button
                        key={t.contract}
                        onClick={() => {
                          setNewToken({ ...newToken, preset: t.contract, contract: t.contract, symbol: t.symbol });
                          setAddTokenStep('amounts');
                        }}
                        style={{
                          padding: '10px',
                          borderRadius: 12,
                          border: '1px solid #ddd',
                          background: '#f9f9f9',
                          cursor: 'pointer',
                          fontWeight: 600,
                          color: '#333',
                          fontSize: 13,
                        }}
                      >
                        {t.label}
                      </button>
                    ))}
                  </div>
                </>
              )}

              {/* PORTFOLIO TOKENS */}
              {tokens.length > 0 && (
                <>
                  <div style={{ fontSize: 12, fontWeight: 700, color: '#666', marginBottom: 6 }}>Portfolio Tokens ({tokens.length})</div>
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, marginBottom: 12 }}>
                    {tokens.map(t => (
                      <button
                        key={t.id}
                        onClick={() => {
                          // TODO: add more of existing token
                          setAddTokenStep('amounts');
                        }}
                        style={{
                          padding: '10px',
                          borderRadius: 12,
                          border: '1px solid #ddd',
                          background: '#f9f9f9',
                          cursor: 'pointer',
                          fontWeight: 600,
                          color: '#333',
                          fontSize: 13,
                        }}
                      >
                        {t.symbol}
                      </button>
                    ))}
                  </div>
                </>
              )}

              {/* CUSTOM CONTRACT */}
              <button
                onClick={() => {
                  setNewToken({ ...newToken, preset: '', contract: '', symbol: '' });
                  setAddTokenStep('contract');
                }}
                style={{
                  width: '100%',
                  padding: '10px',
                  borderRadius: 12,
                  border: '1px solid #ddd',
                  background: '#fff',
                  cursor: 'pointer',
                  color: '#2f7eed',
                  fontWeight: 600,
                  marginBottom: 8
                }}
              >
                ✏️ Enter custom contract
              </button>

              <button
                onClick={() => setShowAddTokenModal(false)}
                style={{
                  width: '100%',
                  padding: '10px',
                  borderRadius: 12,
                  border: '1px solid #ddd',
                  background: '#fff',
                  cursor: 'pointer',
                  color: '#666',
                  fontWeight: 600,
                }}
              >
                Cancel
              </button>
            </div>
          </div>
        );
      })()}

      {/* 🎯 ADD TOKEN CONTRACT INPUT MODAL */}
      {showAddTokenModal && addTokenStep === 'contract' && (() => {
        return (
          <div
            onClick={() => setShowAddTokenModal(false)}
            style={overlayStyle}
          >
            <div
              onClick={(e) => e.stopPropagation()}
              style={modalStyle}
            >
              <div style={titleStyle}>Enter contract address</div>

              <input
                type="text"
                placeholder="0x..."
                value={newToken.contract}
                onChange={(e) => setNewToken({ ...newToken, contract: e.target.value })}
                style={{ ...inputStyle, marginBottom: 12 }}
              />

              <button
                onClick={() => setAddTokenStep('amounts')}
                disabled={!newToken.contract.trim()}
                style={{
                  width: '100%',
                  padding: '10px',
                  borderRadius: 12,
                  border: 'none',
                  background: newToken.contract.trim() ? '#2f7eed' : '#ccc',
                  cursor: newToken.contract.trim() ? 'pointer' : 'not-allowed',
                  color: '#fff',
                  fontWeight: 600,
                  marginBottom: 8
                }}
              >
                Next
              </button>

              <button
                onClick={() => {
                  setShowAddTokenModal(false);
                  setAddTokenStep(null);
                }}
                style={{
                  width: '100%',
                  padding: '10px',
                  borderRadius: 12,
                  border: '1px solid #ddd',
                  background: '#fff',
                  cursor: 'pointer',
                  color: '#666',
                  fontWeight: 600,
                }}
              >
                Cancel
              </button>
            </div>
          </div>
        );
      })()}

      {/* 🎯 ADD TOKEN AMOUNTS MODAL */}
      {showAddTokenModal && addTokenStep === 'amounts' && (() => {
        return (
          <div
            onClick={() => setShowAddTokenModal(false)}
            style={overlayStyle}
          >
            <div
              onClick={(e) => e.stopPropagation()}
              style={modalStyle}
            >
              <div style={titleStyle}>Token: {newToken.symbol || 'Custom'}</div>

              <label style={labelStyle}>Amount (Qty)</label>
              <input
                type="number"
                step="0.01"
                placeholder="0.00"
                value={newToken.amount}
                onChange={(e) => setNewToken({ ...newToken, amount: e.target.value })}
                style={{ ...inputStyle, marginBottom: 12 }}
              />

              <label style={labelStyle}>Invested (USD)</label>
              <input
                type="number"
                step="0.01"
                placeholder="0.00"
                value={newToken.invested}
                onChange={(e) => setNewToken({ ...newToken, invested: e.target.value })}
                style={{ ...inputStyle, marginBottom: 12 }}
              />

              <button
                onClick={() => {
                  addToken();
                  setShowAddTokenModal(false);
                  setAddTokenStep(null);
                }}
                disabled={!newToken.amount || !newToken.invested}
                style={{
                  width: '100%',
                  padding: '10px',
                  borderRadius: 12,
                  border: 'none',
                  background: (newToken.amount && newToken.invested) ? '#27a36a' : '#ccc',
                  cursor: (newToken.amount && newToken.invested) ? 'pointer' : 'not-allowed',
                  color: '#fff',
                  fontWeight: 600,
                  marginBottom: 8
                }}
              >
                Add Token
              </button>

              <button
                onClick={() => setAddTokenStep('select')}
                style={{
                  width: '100%',
                  padding: '10px',
                  borderRadius: 12,
                  border: '1px solid #ddd',
                  background: '#fff',
                  cursor: 'pointer',
                  color: '#666',
                  fontWeight: 600,
                  marginBottom: 8
                }}
              >
                Back
              </button>

              <button
                onClick={() => {
                  setShowAddTokenModal(false);
                  setAddTokenStep(null);
                }}
                style={{
                  width: '100%',
                  padding: '10px',
                  borderRadius: 12,
                  border: '1px solid #ddd',
                  background: '#fff',
                  cursor: 'pointer',
                  color: '#666',
                  fontWeight: 600,
                }}
              >
                Cancel
              </button>
            </div>
          </div>
        );
      })()}
  gap: 10,
};

const titleStyle = {
  textAlign: "center",
  fontSize: 17,
  fontWeight: 700,
  marginBottom: 6,
};

const labelStyle = {
  fontSize: 13,
  fontWeight: 600,
  color: "#444",
};

const inputStyle = {
  width: "100%",
  padding: "10px 12px",
  borderRadius: 12,
  border: "1px solid #ddd",
  background: "#f8f8f8",
  fontSize: 14,
  boxSizing: "border-box",
};

const selectStyle = {
  ...inputStyle,
};

const previewStyle = {
  fontSize: 13,
  background: "#f4f6f8",
  padding: 10,
  borderRadius: 12,
  textAlign: "center",
  color: "#333",
};

const actionsStyle = {
  display: "flex",
  flexDirection: "column",
  gap: 8,
  marginTop: 6,
};

const primaryBtnStyle = {
  padding: "11px",
  borderRadius: 12,
  border: "none",
  background: "#4a90e2",
  color: "#fff",
  fontWeight: 700,
  fontSize: 14,
  cursor: "pointer",
};

const secondaryBtnStyle = {
  padding: "11px",
  borderRadius: 12,
  border: "none",
  background: "#bbb",
  color: "#fff",
  fontWeight: 700,
  fontSize: 14,
  cursor: "pointer",
};
