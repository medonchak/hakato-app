import React, { useState } from 'react';
import { TextField, Button, Typography, Box, CircularProgress } from '@mui/material';
import { fetchAddressAnalytics } from '../api';



const AddressAnalytics = ({ }) => { // <-- передай themeType зверху
  const styles = {};

  const [address, setAddress] = useState('');
  const [analytics, setAnalytics] = useState(null);
  const [loading, setLoading] = useState(false);

  const handleAnalyze = async () => {
    if (!address) return;
    setLoading(true);
    const data = await fetchAddressAnalytics(address);
    setAnalytics(data);
    setLoading(false);
  };

  return (
<>
      <Typography variant="h6" sx={styles.title}>
        Address Analytics
      </Typography>
      <Box sx={{ display: 'flex', gap: 1.2, mt: 1, mb: 1 }}>
        <TextField
          label="Address"
          variant="outlined"
          size="small"
          fullWidth
          value={address}
          onChange={(e) => setAddress(e.target.value)}
          sx={styles.input}
        />
        <Button
          variant="contained"
          sx={styles.button}
          onClick={handleAnalyze}
          disabled={loading}
        >
          {loading ? <CircularProgress size={22} sx={{ color: "#08f7fe" }} /> : 'Analyze'}
        </Button>
      </Box>

      {analytics && (
          <Box
          sx={{
            background: "linear-gradient(135deg, #0f172a, #020617)",
            borderRadius: "14px",
            padding: "16px 20px",
            border: "1px solid rgba(255,255,255,0.08)",
            boxShadow: "0 10px 30px rgba(0,0,0,0.35)",
            display: "flex",
            flexDirection: "column",
            gap: "12px",
          }}
        >
          <Typography
            sx={{
              fontSize: "14px",
              letterSpacing: "1px",
              textTransform: "uppercase",
              color: "#94a3b8",
              fontWeight: 600,
            }}
          >
            Results
          </Typography>

          <Box
            sx={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
            }}
          >
            <Typography sx={{ color: "#60a5fa", fontSize: "14px" }}>
              Tx Count
            </Typography>
            <Typography
              sx={{
                fontSize: "16px",
                fontWeight: 800,
                color: "#f87171",
              }}
            >
              {analytics.total_tx}
            </Typography>
          </Box>

          <Box
            sx={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
            }}
          >
            <Typography sx={{ color: "#f472b6", fontSize: "14px" }}>
              Total Gas Used
            </Typography>
            <Typography
              sx={{
                fontSize: "16px",
                fontWeight: 800,
                color: "#f87171",
              }}
            >
              {analytics.total_gas} ETH
            </Typography>
          </Box>
        </Box>

      )}
   </>
  );
};

export default AddressAnalytics;
