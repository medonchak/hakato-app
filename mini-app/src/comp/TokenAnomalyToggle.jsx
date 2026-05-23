import { Switch } from "antd";
import axios from "axios";
import { useState } from "react";
import { togglePortfolioAnomalyAlerts } from "../api";

export default function TokenAnomalyToggle({portfolioId,enabled: initialEnabled}) {
  const [enabled, setEnabled] = useState(initialEnabled);
  const [loading, setLoading] = useState(false);

  const onToggle = async (value) => {
    setLoading(true);
    try {
        togglePortfolioAnomalyAlerts(portfolioId,value)
        setEnabled(value);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Switch
      checked={enabled}
      loading={loading}
      onChange={onToggle}
      checkedChildren="Alerts ON"
      unCheckedChildren="Alerts OFF"
    />
  );
}
