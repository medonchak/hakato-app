// AlertStream.jsx
import { useEffect, useRef, useState, useMemo } from "react";
import {
  Box,
  Typography,
  List,
  ListItem,
  Chip,
} from "@mui/material";
import { warmStyles as S } from "../dashboardWarmStyles";
import { getAlertsByRule } from "../api";

export default function AlertStream({ ruleId }) {
  const [items, setItems] = useState([]);
  const [lastId, setLastId] = useState(0);
  const wsRef = useRef(null);

  // первинне завантаження
  useEffect(() => {
    if (!ruleId) return;

    getAlertsByRule(ruleId, 0).then((data) => {
      const list = Array.isArray(data) ? data : [];
      setItems(list);

      if (list.length > 0) {
        setLastId(list[list.length - 1].id);
      }
    });
  }, [ruleId]);

  // polling нових
  useEffect(() => {
    if (!ruleId || !lastId) return;

    const timer = setInterval(() => {
      getAlertsByRule(ruleId, lastId).then((data) => {
        const list = Array.isArray(data) ? data : [];
        if (list.length === 0) return;

        setItems((prev) => [...prev, ...list]);
        setLastId(list[list.length - 1].id);
      });
    }, 5000);

    return () => clearInterval(timer);
  }, [ruleId, lastId]);

  // 🔑 сортуємо так, щоб нові були зверху
  const sortedItems = useMemo(() => {
    if (!Array.isArray(items)) return [];
    return [...items].sort(
      (a, b) => new Date(b.created_at) - new Date(a.created_at)
    );
  }, [items]);
function normalizeAlertData(raw) {
  // ----- SWAP FINANCE -----
  if (raw?.swapIns?.length || raw?.swapOuts?.length) {
    const inToken = raw.swapIns?.[0];
    const outToken = raw.swapOuts?.[0];

    // показуємо логічно: що прийшло або що пішло
    const main = inToken || outToken;

    return {
      tokenSymbol: `${inToken?.tokenSymbol || ""} → ${outToken?.tokenSymbol || ""}`,
      amountHuman: main?.amount || 0,
      valueUsd: main?.usd || 0,
      direction: inToken ? "IN" : "OUT",
      from: raw.from,
      to: raw.to,
      _isFinanceSwap: true,
    };
  }

  // ----- NORMAL SWAP -----
  return raw;
}

  return (
    <Box
      sx={{
        p: 2,
        background: S.card,
        borderRadius: 2,
        boxShadow: "0 1.5px 8px #47a6c714",
        border: S.cardBorder,
      }}
    >
      <Typography
        variant="h6"
        sx={{ color: S.accentBlue, fontWeight: 700, mb: 1 }}
      >
        Alerts
      </Typography>

      <List dense>
        {sortedItems.map((a) => {
          let data = null;

          try {
            const raw = a.details ? JSON.parse(a.details) : null;
            data = raw ? normalizeAlertData(raw) : null;
            console.log(data)
          } catch {
            data = null;
          }

          return (
            <ListItem
              key={a.id}
              sx={{
                mb: 0.75,
                px: 1.5,
                py: 1,
                borderLeft: "4px solid #fe53bb",
                background:
                  "linear-gradient(90deg, rgba(254,83,187,0.08), transparent)",
                borderRadius: 1,
              }}
            >
              <Box sx={{ width: "100%" }}>
                {/* TOP */}
                <Box
                  sx={{
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
                  }}
                >
                  {data ? (
                    <>
                      <Box
                        sx={{
                          display: "flex",
                          gap: 1,
                          alignItems: "center",
                        }}
                      >
                        <Chip
                          size="small"
                          label={data.tokenSymbol}
                          sx={{ fontWeight: 600 }}
                        />
                        <Typography fontWeight={600}>
                          {Number(data.amountHuman).toFixed(2)}
                        </Typography>
                        <Typography
                          variant="caption"
                          color="text.secondary"
                        >
                          ${Number(data.valueUsd).toFixed(2)}
                        </Typography>
                      </Box>

                      <Chip
                        size="small"
                        label={data.direction}
                        color={
                          data.direction === "OUT"
                            ? "error"
                            : "success"
                        }
                        sx={{ fontWeight: 600 }}
                      />
                    </>
                  ) : (
                    <Typography fontWeight={600}>
                      Transaction alert
                    </Typography>
                  )}
                </Box>

                {/* ADDRESSES */}
                <Typography
                  variant="caption"
                  sx={{ opacity: 0.8, display: "block" }}
                >
                  {data
                    ? `${data.from.slice(0, 6)}…${data.from.slice(
                        -4
                      )} → ${data.to.slice(0, 6)}…${data.to.slice(-4)}`
                    : a.address}
                </Typography>

                {/* FOOTER */}
                <Box
                  sx={{
                    display: "flex",
                    justifyContent: "space-between",
                    mt: 0.25,
                  }}
                >
                  <Typography
                    variant="caption"
                    sx={{ opacity: 0.6 }}
                  >
                    Tx: {a.tx_hash.slice(0, 10)}…
                    {a.tx_hash.slice(-6)}
                  </Typography>

                  <Typography
                    variant="caption"
                    sx={{ opacity: 0.6 }}
                  >
                    {new Date(a.created_at).toLocaleString()}
                  </Typography>
                </Box>
              </Box>
            </ListItem>
          );
          
        })}
        
      </List>
    </Box>
  );
}
