import React, { useState } from "react";
import { Box, Typography, Divider, Button } from "@mui/material";

const TEXT = {
  en: {
    title: "How to use Crypto Dashboard",
    subtitle: "Simple on-chain analytics & alerts inside Telegram",
    tip: "Tip: For precise alerts, always specify network and minimum value.",
    sections: [
      {
        title: "1. Portfolios",
        lines: [
          "Click “+ Portfolio” and create a new portfolio.",
          "Open it and add tokens by selecting network, contract, amount and invested value.",
          "Portfolio is used for analytics only — your funds are never touched.",
        ],
      },
      {
        title: "2. Alert Rules",
        lines: [
          "Click “+ Rules” and create a rule set.",
          "Add wallet or contract address, choose network and event type.",
          "Set minimum amount and optional token contracts.",
          "Save the rule to activate alerts.",
        ],
      },
      {
        title: "3. Telegram Alerts",
        lines: [
          "When conditions are met, you receive a Telegram notification.",
          "Alert shows event type, rule name, From / To, network, value and transaction link.",
        ],
      },
      {
        title: "4. Security",
        lines: [
          "Mini App never asks for private keys.",
          "All data is read-only and fetched directly from blockchain.",
          "Everything is linked to your Telegram account.",
        ],
      },
      {
        title: "5. Wallet Analytics (Profile)",
        lines: [
          "Click on your user icon to open the wallet analytics panel.",
          "Enter any wallet address to analyze on-chain activity.",
          "You can see total transaction count and total gas used.",
          "Analytics is calculated for activity after Ethereum switched to Proof of Stake.",
        ],
      },
    ],
  },

  ua: {
    title: "Як користуватися Crypto Dashboard",
    subtitle: "Ончейн-аналітика та алерти прямо в Telegram",
    tip: "Порада: для точних алертів вказуй мережу та мінімальну суму.",
    sections: [
      {
        title: "1. Портфелі",
        lines: [
          "Натисни “+ Portfolio” та створи портфель.",
          "Відкрий його і додай токени: мережа, контракт, кількість та інвестиція.",
          "Портфель використовується лише для аналітики — кошти не зачіпаються.",
        ],
      },
      {
        title: "2. Алерти (Rules)",
        lines: [
          "Натисни “+ Rules” та створи набір правил.",
          "Додай адресу гаманця або контракт, обери мережу та тип події.",
          "Вкажи мінімальну суму та за потреби токени.",
          "Збережи правило для активації алертів.",
        ],
      },
      {
        title: "3. Telegram-сповіщення",
        lines: [
          "Коли умова виконується — ти отримуєш повідомлення в Telegram.",
          "Алерт містить тип події, правило, From / To, мережу, суму та посилання.",
        ],
      },
      {
        title: "4. Безпека",
        lines: [
          "Mini App ніколи не запитує приватні ключі.",
          "Усі дані лише зчитуються з блокчейна.",
          "Дані прив’язані до твого Telegram-акаунту.",
        ],
      },
      {
        title: "5. Аналітика гаманця (Профіль)",
        lines: [
          "Натисни на іконку користувача, щоб відкрити панель аналітики.",
          "Введи адресу гаманця для аналізу ончейн-активності.",
          "Ти побачиш кількість транзакцій та загальний використаний gas.",
          "Аналітика рахується для періоду після переходу Ethereum на Proof of Stake.",
        ],
      },
    ],
  },
};

const HowToUse = () => {
  const [lang, setLang] = useState("en");
  const t = TEXT[lang];

  return (
    <Box
      sx={{
        maxWidth: 760,
        margin: "0 auto",
        padding: "28px 20px",
        background: "linear-gradient(180deg, #fffaf3, #f5f7f9)",
        borderRadius: "20px",
        boxShadow: "0 20px 50px rgba(0,0,0,0.12)",
      }}
    >
      {/* LANGUAGE SWITCH */}
      <Box sx={{ display: "flex", justifyContent: "flex-end", mb: 2 }}>
        <Button
          size="small"
          variant={lang === "en" ? "contained" : "outlined"}
          onClick={() => setLang("en")}
          sx={{ mr: 1 }}
        >
          EN
        </Button>
        <Button
          size="small"
          variant={lang === "ua" ? "contained" : "outlined"}
          onClick={() => setLang("ua")}
        >
          UA
        </Button>
      </Box>

      {/* HEADER */}
      <Typography sx={{ fontSize: 28, fontWeight: 800, textAlign: "center" }}>
        {t.title}
      </Typography>

      <Typography sx={{ textAlign: "center", color: "#64748b", mb: 4 }}>
        {t.subtitle}
      </Typography>

      <Divider sx={{ mb: 4 }} />

      {/* CONTENT */}
      {t.sections.map((s, i) => (
        <Section key={i} title={s.title} lines={s.lines} />
      ))}

      <Divider sx={{ mt: 4, mb: 3 }} />

      <Typography sx={{ textAlign: "center", fontSize: 13, color: "#94a3b8" }}>
        {t.tip}
      </Typography>
    </Box>
  );
};

/* ===========================================================
   SECTION
   =========================================================== */
const Section = ({ title, lines }) => (
  <Box sx={{ mb: 3 }}>
    <Typography sx={{ fontSize: 18, fontWeight: 700, mb: 1 }}>
      {title}
    </Typography>
    {lines.map((l, i) => (
      <Typography key={i} sx={{ fontSize: 14, color: "#475569", lineHeight: 1.7 }}>
        • {l}
      </Typography>
    ))}
  </Box>
);

export default HowToUse;
