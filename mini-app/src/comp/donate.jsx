import React from "react";

const Donate = () => {
    const walletAddressERC = "0x486E2f6230534780987bf5E5AEe03b90d5F3f7a9";
    const walletAddressTRC = "TKTFYZQrRCszrZq4qorLHr93u9d7vtZTGR";


  return (
    <div style={styles.wrapper}>
      <h2 style={styles.title}>Support the Project</h2>

      <p style={styles.text}>
        This project is developed independently and focuses on real-time
        on-chain analytics, alerts, and clear blockchain insights.
      </p>

      <p style={styles.text}>
        Community support helps maintain infrastructure, improve analytics,
        and accelerate development of new features.
      </p>

    <div style={styles.walletBox}>
      <WalletRow label="ERC20 (EVM)" value={walletAddressERC} />
      <WalletRow label="TRC20" value={walletAddressTRC} />
    </div>

      <p style={styles.note}>
        Supporters may receive early access, priority features, or additional
        benefits as the platform evolves.
      </p>
    </div>
  );
};

const styles = {
  wrapper: {
    maxWidth: "520px",
    margin: "0 auto",
    padding: "32px 20px",
    textAlign: "center",
  },
  title: {
    marginBottom: "16px",
  },
  text: {
    marginBottom: "12px",
    opacity: 0.85,
  },

  label: {
    display: "block",
    marginBottom: "6px",
    fontSize: "12px",
    opacity: 0.6,
  },
walletBox: {
  margin: "24px auto 0",
  padding: "16px",
  maxWidth: "520px",
  border: "1px solid rgba(255,255,255,0.18)",
  borderRadius: "10px",
  background: "rgba(255,255,255,0.04)",
},

walletRow: {
  display: "flex",
  justifyContent: "space-between",
  alignItems: "center",
  gap: "12px",
  padding: "10px 8px",
  cursor: "pointer",
  borderRadius: "6px",
},

network: {
  fontSize: "13px",
  opacity: 0.7,
  whiteSpace: "nowrap",
},

wallet: {
  fontSize: "14px",
  wordBreak: "break-all",
  textAlign: "right",
},

// optional hover effect
walletRowHover: {
  background: "rgba(255,255,255,0.06)",
},

    note: {
        marginTop: "20px",
        fontSize: "13px",
        opacity: 0.6,
    },
};
const WalletRow = ({ label, value }) => {
  const [copied, setCopied] = React.useState(false)
const copyText = async (text, setCopied) => {
  try {
    if (window.Telegram?.WebApp?.clipboard) {
      await window.Telegram.WebApp.clipboard.writeText(text)
    } else if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
    }

    setCopied(true)
    setTimeout(() => setCopied(false), 1200)
  } catch (e) {
    console.error("Copy failed", e)
  }
}
  return (
    <div
      style={{
        ...styles.walletRow,
        background: copied ? "rgba(0,200,120,0.12)" : "transparent",
        transition: "0.2s",
        cursor: "pointer",
      }}
      onClick={() => copyText(value, setCopied)}
    >
      <span style={styles.network}>{label}</span>

      <code style={{
        ...styles.wallet,
        color: copied ? "#00c878" : styles.wallet.color,
      }}>
        {copied ? "Copied ✔" : value}
      </code>
    </div>
  )
}
export default Donate;
