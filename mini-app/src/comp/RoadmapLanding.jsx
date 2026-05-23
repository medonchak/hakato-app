import React from "react";
import { Box, Typography } from "@mui/material";

const steps = [
  {
    title: "Ethereum Core",
    desc: "Blocks, transactions and gas collection. The foundational on-chain data layer.",
    color: "#5B8CFF",
    side: "left",
  },
  {
    title: "Alerts Engine",
    desc: "Rules, filters and triggers to monitor critical on-chain events.",
    color: "#4CC9A7",
    side: "right",
  },
  {
    title: "Telegram Layer",
    desc: "Telegram Mini App integration and real-time alert delivery.",
    color: "#8E9EFF",
    side: "left",
  },
  {
    title: "Portfolio System",
    desc: "Portfolio tracking, tokens, P&L calculation and performance control.",
    color: "#F2C94C",
    side: "right",
  },
  {
    title: "AI Pattern Engine",
    desc: "Detection of behavioral patterns across wallets and transactions.",
    color: "#56CCF2",
    side: "left",
  },
  {
    title: "Whale & Risk Detection",
    desc: "Monitoring large wallets, anomalies and potential market manipulation.",
    color: "#EB5757",
    side: "right",
  },
  {
    title: "DEX & NFT Analytics",
    desc: "Liquidity pools, swaps, mints and NFT market activity analysis.",
    color: "#27AE60",
    side: "left",
  },
  {
    title: "AI Copilot",
    desc: "Human-readable explanations of complex on-chain activity.",
    color: "#BB6BD9",
    side: "right",
  },
  {
    title: "Multi-Chain & API",
    desc: "Expansion to multiple EVM networks and Enterprise-grade API access.",
    color: "#2F80ED",
    side: "left",
  },
];

export default function RoadmapInfographic() {
  return (
    <Box
      sx={{
        bgcolor: "#F7F9FC",
        py: 8,
        px: 2,
        minHeight: "100vh",
      }}
    >
      <Box sx={{ maxWidth: 900, mx: "auto" }}>
        {/* Header */}
        <Typography
          sx={{
            fontSize: 30,
            fontWeight: 900,
            textAlign: "center",
            color: "#1E293B",
            mb: 1,
          }}
        >
          Product Roadmap
        </Typography>

        <Typography
          sx={{
            textAlign: "center",
            color: "#64748B",
            mb: 8,
          }}
        >
          Visual roadmap of the on-chain analytics platform architecture
        </Typography>

        {/* Timeline */}
        <Box sx={{ position: "relative" }}>
          {/* Central line */}
          <Box
            sx={{
              position: "absolute",
              left: "50%",
              top: 0,
              bottom: 0,
              width: 6,
              background:
                "linear-gradient(180deg, #CBD5E1 0%, #E2E8F0 100%)",
              transform: "translateX(-50%)",
            }}
          />

          {steps.map((step, i) => (
            <Box
              key={i}
              sx={{
                display: "flex",
                justifyContent:
                  step.side === "left" ? "flex-start" : "flex-end",
                mb: 7,
                position: "relative",
              }}
            >
              {/* Text block */}
              <Box
                sx={{
                  width: "42%",
                  textAlign: step.side === "left" ? "right" : "left",
                  px: 2,
                }}
              >
                <Typography
                  sx={{
                    fontWeight: 800,
                    fontSize: 16,
                    color: "#1E293B",
                    mb: 0.5,
                    ml: 2,
                    mr: 2,
                  }}
                >
                  {step.title}
                </Typography>

                <Typography
                  sx={{
                    fontSize: 14,
                    color: "#475569",
                    lineHeight: 1.5,
                    ml: 2,
                    mr: 2,
                  }}
                >
                  {step.desc}
                </Typography>
              </Box>

              {/* Node */}
              <Box
                sx={{
                  position: "absolute",
                  left: "50%",
                  top: 6,
               
                  transform: "translateX(-50%)",
                  width: 28,
                  height: 28,
                  borderRadius: "50%",
                  bgcolor: step.color,
                  border: "6px solid #F7F9FC",
                  boxShadow: "0 6px 20px rgba(0,0,0,0.15)",
                }}
              />
            </Box>
          ))}
        </Box>
      </Box>
    </Box>
  );
}
