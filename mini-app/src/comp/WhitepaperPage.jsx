import React from "react";
import { Box, Typography } from "@mui/material";


export default function WhitepaperArticle() {
  return (
    <Box
      sx={{
        bgcolor: "#F8FAFC",
        color: "#0F172A",
        minHeight: "100vh",
        py: { xs: 4, md: 8 },
        px: { xs: 2, md: 4 },
        display: "flex",
        justifyContent: "center",
      }}
    >
      
      <Box
        sx={{
          width: "100%",
          maxWidth: 860,
        }}
      >
        {/* TITLE */}
        <Typography
          sx={{
            fontSize: { xs: 28, md: 36 },
            fontWeight: 900,
            mb: 4,
            textAlign: "center",
          }}
        >
          On-Chain Analytics Mini App
        </Typography>

        {/* SECTION 1 */}
        <Section title="1. Introduction">
          <P>
            Blockchain networks generate vast amounts of transparent but highly
            complex data. While all transactions are public, extracting
            actionable insights from raw on-chain data remains difficult for
            most users.
          </P>

          <P>
            This project introduces a lightweight on-chain analytics platform,
            delivered as a Telegram Mini App, designed to transform raw
            blockchain data into clear signals, alerts, and explanations for
            both retail users and professionals.
          </P>

          <P>
            The platform focuses on real-time analysis, behavioral patterns, and
            risk detection, without overwhelming users with unnecessary
            complexity.
          </P>
        </Section>

        {/* SECTION 2 */}
        <Section title="2. Problem Statement">
          <UL>
            <LI>Raw blockchain data is difficult to interpret</LI>
            <LI>
              Existing analytics platforms are complex, expensive, or
              overloaded with features
            </LI>
            <LI>Important on-chain events often go unnoticed</LI>
            <LI>
              Behavioral patterns of wallets and smart contracts are hard to
              detect manually
            </LI>
            <LI>
              Most tools are not optimized for real-time alerting and mobile
              usage
            </LI>
          </UL>

          <P>
            There is a strong demand for a simple, fast, and intelligent
            on-chain monitoring solution.
          </P>
        </Section>

        {/* SECTION 3 */}
        <Section title="3. Solution Overview">
          <P>
            The proposed solution is a modular on-chain analytics platform that:
          </P>

          <UL>
            <LI>Collects blockchain data in real time</LI>
            <LI>Normalizes and structures events</LI>
            <LI>Detects meaningful patterns and anomalies</LI>
            <LI>Delivers insights directly to users via Telegram</LI>
          </UL>

          <P>
            The platform is designed as a progressively expandable system,
            starting from a minimal core and scaling into advanced analytics and
            enterprise integrations.
          </P>
        </Section>

        {/* SECTION 4 */}
        <Section title="4. Product Architecture">
          <P>
            The system is built around a data flow architecture, where each
            module processes and enriches information before passing it
            forward.
          </P>

          <SubTitle>4.1 Ethereum Core</SubTitle>
          <UL>
            <LI>Collects blocks, transactions, and gas data</LI>
            <LI>Serves as the primary source of raw on-chain information</LI>
            <LI>Ensures data accuracy and continuity</LI>
          </UL>

          <SubTitle>4.2 Alerts Engine</SubTitle>
          <UL>
            <LI>Applies rules and filters to on-chain events</LI>
            <LI>Detects predefined and dynamic conditions</LI>
            <LI>Generates alerts in real time</LI>
          </UL>

          <SubTitle>4.3 Telegram Layer</SubTitle>
          <UL>
            <LI>Telegram Mini App as the main user interface</LI>
            <LI>Instant delivery of alerts and notifications</LI>
            <LI>Optimized for mobile-first interaction</LI>
          </UL>

          <SubTitle>4.4 Portfolio System</SubTitle>
          <UL>
            <LI>Tracks wallets and token balances</LI>
            <LI>Calculates P&amp;L and performance metrics</LI>
            <LI>Provides contextual analytics per user</LI>
            <LI>Performs on-chain analytics of token activity across the network</LI>
            <LI>Monitors different types of token activity, including transfers, large movements, smart contract interactions, swaps, and liquidity changes</LI>
            <LI>Enables real-time notifications when significant, unusual, or user-defined activity occurs for tokens held in the user’s portfolio</LI>
          </UL>

          <SubTitle>4.5 AI Pattern Engine</SubTitle>
          <UL>
            <LI>Identifies behavioral patterns of wallets</LI>
            <LI>Detects abnormal or coordinated activity</LI>
            <LI>Classifies events by relevance and risk</LI>
          </UL>

          <SubTitle>4.6 Risk & Whale Detection</SubTitle>
          <UL>
            <LI>Monitors large wallet movements</LI>
            <LI>Detects potential manipulation and anomalies</LI>
            <LI>Flags suspicious activity early</LI>
          </UL>

          <SubTitle>4.7 DEX & NFT Analytics</SubTitle>
          <UL>
            <LI>Analyzes liquidity pools and swaps</LI>
            <LI>Tracks NFT minting and transfers</LI>
            <LI>Identifies market activity trends</LI>
          </UL>

          <SubTitle>4.8 AI Copilot</SubTitle>
          <UL>
            <LI>
              Translates complex on-chain activity into human-readable
              explanations
            </LI>
            <LI>
              Assists users in understanding alerts and signals
            </LI>
            <LI>Reduces the learning curve for non-technical users</LI>
          </UL>

          <SubTitle>4.9 Multi-Chain Expansion & API</SubTitle>
          <UL>
            <LI>Expansion to multiple EVM-compatible networks</LI>
            <LI>Unified data structure across chains</LI>
            <LI>Enterprise-grade API for external integrations</LI>
          </UL>
        </Section>

        {/* SECTION 5 */}
        <Section title="5. Roadmap Philosophy">
          <UL>
            <LI>Stability of the core data layer</LI>
            <LI>Reliable real-time alerting</LI>
            <LI>Intelligent pattern detection</LI>
            <LI>Scalable multi-chain architecture</LI>
          </UL>

          <P>
            This approach ensures that each stage builds upon a proven and
            stable foundation, minimizing technical debt and system fragility.
          </P>
        </Section>

        {/* SECTION 6 */}
        <Section title="6. Target Users">
          <UL>
            <LI>Crypto traders and investors</LI>
            <LI>On-chain analysts</LI>
            <LI>DeFi and NFT participants</LI>
            <LI>Developers and researchers</LI>
            <LI>
              Users who want on-chain insights without complex dashboards
            </LI>
          </UL>
        </Section>

        {/* SECTION 7 */}
        <Section title="7. Key Advantages">
          <UL>
            <LI>Real-time on-chain monitoring</LI>
            <LI>Mobile-first delivery via Telegram</LI>
            <LI>Modular and scalable architecture</LI>
            <LI>Clear explanations instead of raw data</LI>
            <LI>Lightweight and fast user experience</LI>
          </UL>
        </Section>

        {/* SECTION 8 */}
        <Section title="8. Future Vision">
          <UL>
            <LI>Explaining blockchain activity in real time</LI>
            <LI>Detecting emerging market behavior</LI>
            <LI>Serving as an analytical backend for third-party products</LI>
            <LI>Providing reliable data access for enterprise users</LI>
          </UL>
        </Section>

        {/* SECTION 9 */}
        <Section title="9. Conclusion">
          <P>
            This project bridges the gap between blockchain transparency and
            human understanding.
          </P>

          <P>
            By combining real-time data, intelligent analysis, and a simple
            delivery channel, the platform empowers users to make informed
            decisions in an increasingly complex on-chain environment.
          </P>
        </Section>
      </Box>
    </Box>
  );
}

/* ---------- helpers ---------- */

function Section({ title, children }) {
  return (
    <Box sx={{ mb: 6 }}>
      <Typography
        sx={{
          fontSize: 22,
          fontWeight: 800,
          mb: 2,
        }}
      >
        {title}
      </Typography>
      {children}
    </Box>
  );
}

function SubTitle({ children }) {
  return (
    <Typography
      sx={{
        fontWeight: 700,
        mt: 3,
        mb: 1,
      }}
    >
      {children}
    </Typography>
  );
}

function P({ children }) {
  return (
    <Typography
      sx={{
        fontSize: 16,
        lineHeight: 1.7,
        mb: 2,
        color: "#334155",
      }}
    >
      {children}
    </Typography>
  );
}

function UL({ children }) {
  return (
    <Box component="ul" sx={{ pl: 3, mb: 2 }}>
      {children}
    </Box>
  );
}

function LI({ children }) {
  return (
    <Typography
      component="li"
      sx={{
        fontSize: 16,
        lineHeight: 1.6,
        mb: 1,
        color: "#334155",
      }}
    >
      {children}
    </Typography>
  );
}
