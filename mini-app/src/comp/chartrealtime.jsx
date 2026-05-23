import React, { useEffect, useRef, useState } from 'react';
import { Paper, Typography, Box, Select, MenuItem, FormControl, InputLabel } from '@mui/material';
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid } from 'recharts';

// Таймфрейм у хвилинах
const TIMEFRAMES = [
  { label: '1 хв', value: 1 },
  { label: '5 хв', value: 5 },
  { label: '15 хв', value: 15 },
  { label: '60 хв', value: 60 },
];

const POINTS = 14; // Скільки точок хочеш бачити на графіку (20 — приблизно остання година для 3хв фрейму)

function genTimeLabel(date) {
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

// Округлення часу до таймфрейму
function roundToTimeframe(ts, timeframeMin) {
  const date = new Date(ts);
  date.setSeconds(0, 0);
  const minutes = date.getMinutes();
  date.setMinutes(minutes - (minutes % timeframeMin));
  return date.getTime();
}

// Агрегація + "заповнення"
function aggregateData(rawData, timeframe, points = POINTS) {
    
  const stepMs = timeframe * 60 * 1000;
  // Час останнього повного інтервалу
  const now = Date.now();
  const lastSlot = roundToTimeframe(now, timeframe);

  // Формуємо всі часові точки назад від останньої (чітко по таймфрейму!)
  const timestamps = [];
  for (let i = points - 1; i >= 0; i--) {
   
    timestamps.push(lastSlot - i * stepMs);
  }

  // Групуємо rawData по цим слотам
  const grouped = {};
  rawData.forEach(d => {
    const rounded = roundToTimeframe(d.timestamp, timeframe);
    if (!grouped[rounded]) grouped[rounded] = [];
    grouped[rounded].push(d.txCount);
  });

  // Малюємо рівно POINTS підписів часу, навіть якщо нема даних — txCount=0
  return timestamps.map(ts => {
    const arr = grouped[ts] || [];
    return {
      time: genTimeLabel(new Date(ts)),
      txCount: arr.reduce((sum, v) => sum + v, 0)
    }
  });
}


const ChartRealtime = ({ txCount, sx = {} }) => {
  const [timeframe, setTimeframe] = useState(1);
  const [rawData, setRawData] = useState([]);
  const prevTx = useRef();

  // Зберігаємо всі "сирі" значення (кожен update)
  useEffect(() => {
    if (typeof txCount !== "number") return;
    const now = Date.now();
    if (prevTx.current === txCount && rawData.length > 0) return;
    setRawData(prev =>
      [
        ...prev,
        { timestamp: now, txCount }
      ].filter(d => now - d.timestamp < 70 * 60 * 1000) // максимум 70 хв історії
    );
    prevTx.current = txCount;
    // eslint-disable-next-line
  }, [txCount]);

  const data = aggregateData(rawData, timeframe, POINTS);
 
  return (
    <Paper elevation={3} sx={{ p: 2.5, ...sx }}>
      <Box sx={{ display: 'flex', alignItems: 'center', mb: 2, justifyContent: 'space-between' }}>
        <Typography variant="subtitle1">Транзакції в реальному часі</Typography>
        <FormControl size="small" sx={{ minWidth: 90 }}>
          <InputLabel>Таймфрейм</InputLabel>
          <Select
            value={timeframe}
            label="Таймфрейм"
            onChange={e => setTimeframe(Number(e.target.value))}
          >
            {TIMEFRAMES.map(opt => (
              <MenuItem key={opt.value} value={opt.value}>{opt.label}</MenuItem>
            ))}
          </Select>
        </FormControl>
      </Box>
      <ResponsiveContainer width="100%" minWidth={20} height={180}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis  dataKey="time" />
          <YAxis allowDecimals={false} />
          <Tooltip
            formatter={(v, n, props) =>
              props.dataKey === "txCount"
                ? [`${v} tx за ${timeframe} хв`, 'Tx Count']
                : v
            }
          />
          <Line type="monotone" dataKey="txCount" stroke="#47a6c7" strokeWidth={3} dot={false} />
        </LineChart>
      </ResponsiveContainer>
    </Paper>
  );
};

export default ChartRealtime;
