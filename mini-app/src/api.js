// src/api.js

import axios from 'axios';

export const api = axios.create({ baseURL: "/api" });


export const fetchAddressAnalytics = async (address) => {
  try {
    const response = await api.post('/address-analytics', { address });
    return response.data;
  } catch (error) {
    console.error('❌ Error fetching address analytics:', error);
    return null;
  }
};
export const getUserAlerts = async (telegramId) => {
  try {
   
    const response = await api.get(`/alert-rules`, {
      params: { telegram_id: telegramId },
    });
    return response.data;
  } catch (error) {
    console.error('❌ Error fetching user alerts:', error);
    return [];
  }
};
export const sendAlerts = async (payload) => {
  try {
    const response = await api.post('/alert-rule', { payload });
    return response.data;
  } catch (error) {
    const message =
      error?.response?.data ||
      error?.message ||
      'Server error';
    console.error('❌ Error sendAlerts:', error);
    return { ok: false, error: String(message) };
  }
};
export const sendToken = async (list) => {
  try {
 
    const response = await api.post('/token-prices',  list );

    return response.data;
  } catch (error) {
    console.error('❌ Error sendAlerts:', error);
    return null;
  }
};

/* ================================
   🆕 2) НОВІ API ФУНКЦІЇ
   ================================ */

/* -----------  USER INIT  ------------- */
export const initUser = async (telegramId) => {
  try {
    const response = await api.post("/init-user", { telegram_id: telegramId });
    return response.data;
  } catch (error) {
    console.error("❌ Error initUser:", error);
    return null;
  }
};

/* -----------  PORTFOLIO CREATE  ----------- */
export const createPortfolio = async (userId, name) => {
  try {
    const response = await api.post("/portfolios/create", {
      userId,
      name,
    });
    return response.data;
  } catch (error) {
    console.error("❌ Error createPortfolio:", error);
    return null;
  }
};

/* -----------  GET USER PORTFOLIOS  ----------- */
export const getPortfolios = async (telegramId) => {
  try {
    const response = await api.get("/portfolios", {
      params: { user_id: telegramId },
    });

    return response.data;
  } catch (error) {
    console.error("❌ Error getPortfolios:", error);
    return [];
  }
};

/* -----------  ADD TOKEN TO PORTFOLIO  ----------- */
export const addTokenToPortfolio = async (payload) => {

  try {
  console.log(payload)
    const response = await api.post("/portfolio/token/add", payload);
    return response.data;
  } catch (error) {
    console.error("❌ Error addTokenToPortfolio:", error);
    return null;
  }
};

export const portfolioOperation = async (payload) => {
  try {
    const response = await api.post("/portfolio/operation", payload);
    return response.data;
  } catch (error) {
    console.error("❌ Error portfolioOperation:", error);
    return null;
  }
};



// створення правила (тільки назва + telegram_id)
export const createRuleName = async (telegramId, name) => {
  try {
 
    const resp = await api.post("/alert-rule/name", {
      telegram_id: telegramId,
      name,
    });
    return resp.data; // { ruleId: ... }
  } catch (error) {
    console.error("❌ Error createRuleName:", error);
    return null;
  }
};

// отримати всі правила (картки) юзера по його user_id (з таблиці users)
export const getAlertRules = async (userId) => {
  try {
    const resp = await api.get("/alert-rules", {
      params: { user_id: userId },
    });
   
    return resp.data; // масив AlertRuleCard
  } catch (error) {
    console.error("❌ Error getAlertRules:", error);
    return [];
  }
};
export const getRuleFilters = async (ruleId) => {
  try {
    const res = await api.get("/alert-rule/filters", {
      params: { rule_id: ruleId },
    });

    return res.data; // масив фільтрів
  } catch (e) {
    console.error("❌ Error getRuleFilters:", e);
    return [];
  }
};
export const getPortfolioTokens = async (portfolioId) => {
  try {
    const res = await api.get("/portfolio/tokens", {
      params: { portfolio_id: portfolioId },
    });

    return res.data;
  } catch (e) {
    console.error("❌ Error getPortfolioTokens:", e);
    return [];
  }
};
export const getPortfolioAssets = async (portfolioId) => {
  try {
    const res = await api.get("/portfolio/assets", {
      params: { portfolio_id: portfolioId },
    });
    return res.data;
  } catch (e) {
    console.error("❌ Error getPortfolioAssets:", e);
    return [];
  }
};
export const updatePortfolioAssetTracking = async (payload) => {
  const { data } = await api.post("/portfolio/asset-tracking", payload);
  return data;
};
export const updatePortfolioAssetNetworkTracking = async (payload) => {
  const { data } = await api.post("/portfolio/asset-tracking/network", payload);
  return data;
};
export const getPortfolioNotifications = async (portfolioId) => {
  try {
    const res = await api.get(`/portfolio/${portfolioId}/notifications`);
    return res.data;
  } catch (e) {
    console.error("❌ Error getPortfolioNotifications:", e);
    return [];
  }
};
export const getAlertsByRule = async (ruleId, afterId = 0) => {
  if (!ruleId) return [];
  const res = await api.get("/alerts/by-rule", {
    params: {
      rule_id: ruleId,
      after_id: afterId,
    },
  });

  return res.data; // масив alerts
};
export const deleteAlertFilter = async (id) => {
  await api.delete("/alert-filter", { params: { id } });
};
export const updateAlertFilter = async (id, filter) => {

  await api.put("/alert-filter", { id, filter });
};
export const deleteAlertRule = async (id) => {
  await api.delete("/alert-rule", { params: { id } });
};
export const deletePortfolio = async (id) => {
  await api.delete("/portfolio", { params: { id } });
};



export async function fetchTopTokens(chainId) {
  const { data } = await api.get("/onchain/tokens/top", {
    params: { chain_id: chainId },
  });
  return data;
}

/**
 * Дашборд токена в портфелі
 */
export async function fetchTokenDashboard(portfolioId, chainId, token) {
  const { data } = await api.get(
    `/portfolio/${portfolioId}/token/${token}`,
    {
      params: { chain_id: chainId },
    }
  );
  return data;
}

/**
 * Увімкнути / вимкнути денний звіт або алерти
 */
export async function toggleTokenSubscription(payload) {
  const { data } = await api.post(
    "/token/subscription/daily-report",
    payload
  );
  return data;
}
export async function togglePortfolioAnomalyAlerts(portfolioId, enabled) {
  return api.post("/portfolio/anomaly-alerts", {
    portfolio_id: portfolioId,
    enabled,
  });
}

export async function getTokenActivity(id, limit = 50) {
  const { data } = await api.get("/token/activity", {
    params: {
       token_id: id,
      limit,
    },
  });
  return data;
}

export async function getTokenHourly(id) {
  const { data } = await api.get("/token/hourly", {
    params: {
      token_id: id,
    },
  });
  return data;
}

export async function getTokenAnomalies(id) {
  const { data } = await api.get("/token/anomalies", {
    params: {
      token_id: id,
    },
  });
  return data;
}

// ===== MARKET =====

export async function getTopActiveTokens(id) {
  
  const { data } = await api.get("/market/top-active");
  return data;
}
export async function getMarketTokenActivity(params = {}) {
  try {
    const { data } = await api.get("/market/token-activity", { params });
    return data;
  } catch (error) {
    if (error?.response?.status === 404) {
      const { data } = await api.get("/market/top-active");
      const list = Array.isArray(data) ? data : [];
      return list.map((item) => ({
        ...item,
        tx_count: null,
        unique_addresses: null,
        _fallback_partial: true,
      }));
    }
    throw error;
  }
}
export async function getTokenDashboard(chainId, token) {
  try {
    const { data } = await api.get(`/token/dashboard?chainId=${chainId}&token=${token}`);
    return data;
  } catch (error) {
    if (error?.response?.status === 404) {
      return {
        token,
        chain: chainId,
        state_label: 'NO DATA',
        signal_strength: 'none',
        flow_narrative: 'No on-chain transfers were indexed for this token in the selected period (1h/24h).',
        risk_reasons: [],
        scores: { health: 0, risk: 0, activity: 0 },
        interpretation: {
          transfer_sizes: 'Not enough USD-enriched transfers yet to estimate transfer-size behavior.',
          exchange_flow: 'No exchange flow data for the selected period.',
        },
        facts: {
          active_addresses: 0,
          tx_count: 0,
          exchange_ratio: 0,
          net_exchange: 0,
          p50_token_qty: '--',
          p95_token_qty: '--',
          p99_token_qty: '--',
          data_note: 'No on-chain transfers were indexed for this token in the selected period (1h/24h).',
        },
      };
    }
    throw error;
  }
}







/* -----------  CREATE RULE NAME (empty rule) ----------- */
// export const createRuleName = async (telegramId, name) => {
//   try {
//     console.log({id:telegramId, nameAl:name})
//     const response = await api.post("/alert-rule/name", {
//       telegram_id: telegramId,
//       name,
//     });
//     return response.data;
//   } catch (error) {
//     console.error("❌ Error createRuleName:", error);
//     return null;
//   }
// };

/* -----------  CREATE FULL ALERT RULE  ----------- */
// export const createAlertRule = async (ruleData) => {
//   try {
//     const response = await api.post("/alert-rule", ruleData);
//     return response.data;
//   } catch (error) {
//     console.error("❌ Error createAlertRule:", error);
//     return null;
//   }
// };

/* -----------  GET USER ALERT RULES  ----------- */
// export const getAlertRules = async (telegramId) => {
//   try {
//     const response = await api.get("/alert-rules", {
//       params: { telegram_id: telegramId },
//     });

//     return response.data;
//   } catch (error) {
//     console.error("❌ Error getAlertRules:", error);
//     return [];
//   }
// };

// /* -----------  GET TOKEN PRICES  ----------- */
// export const getTokenPrices = async (contracts) => {
//   // contracts має бути масивом строк
//   try {
//     const response = await api.post("/token-prices", contracts);
//     return response.data;
//   } catch (error) {
//     console.error("❌ Error getTokenPrices:", error);
//     return [];
//   }
// };


// export const connectDashboardWebSocket = (onMessageCallback, alertAddresses) => {
//   const socket = new WebSocket(`${API_BASE_WS}/ws`); // API_BASE_WS = ws://localhost:8080
  
//   socket.onopen = () => {
//     console.log("WebSocket Connected");
//       if (alertAddresses.length > 0){
//         console.log(alertAddresses)
//     socket.send(JSON.stringify({
//         type: "update_addresses",
//         addresses: alertAddresses
//       }));
//   }
//   };

//   socket.onmessage = (event) => {
//     if (onMessageCallback) {
//       onMessageCallback(JSON.parse(event.data));
//     }
//   };

//   socket.onclose = () => {
//     console.log("WebSocket Closed");
//   };

//   socket.onerror = (error) => {
//     console.error("WebSocket Error:", error);
//   };
// };
