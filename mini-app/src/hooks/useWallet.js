/* global BigInt */
import { useState, useEffect, useCallback } from 'react';

const MANTLE_CHAIN_ID = '0x1388'; // 5000 decimal

const MANTLE_NETWORK = {
  chainId: MANTLE_CHAIN_ID,
  chainName: 'Mantle',
  nativeCurrency: { name: 'MNT', symbol: 'MNT', decimals: 18 },
  rpcUrls: [
    'https://rpc.mantle.xyz',
    'https://mantle.publicnode.com',
    'https://mantle-mainnet.public.blastapi.io',
  ],
  blockExplorerUrls: ['https://explorer.mantle.xyz'],
};

const TOKEN_CONTRACTS = {
  mETH: { address: '0xcDA86A272531e8640cD7F1a92c01839911B90bb0', decimals: 18 },
  USDY: { address: '0x5bE26527e817998A7206475496fDE1E68957c5A6', decimals: 18 },
  USDC: { address: '0x09Bc4E0D864854c6aFB6eB9A9cdF58aC190D0dF9', decimals: 6  },
  USDT: { address: '0x201EBa5CC46D216Ce6DC03F6a759e8E766e956aE', decimals: 6  },
};

// ERC-20 balanceOf(address) selector
const BALANCE_OF_SELECTOR = '0x70a08231';

function hexToDecimal(hex) {
  return hex ? BigInt(hex) : BigInt(0);
}

function formatUnits(rawBigInt, decimals) {
  const divisor = BigInt(10 ** decimals);
  const whole = rawBigInt / divisor;
  const frac  = rawBigInt % divisor;
  const fracStr = frac.toString().padStart(decimals, '0').slice(0, 4);
  return `${whole}.${fracStr}`;
}

async function erc20Balance(provider, tokenAddress, walletAddress) {
  // pad wallet address to 32 bytes
  const padded = walletAddress.replace('0x', '').padStart(64, '0');
  const data = BALANCE_OF_SELECTOR + padded;

  const result = await provider.request({
    method: 'eth_call',
    params: [{ to: tokenAddress, data }, 'latest'],
  });
  return hexToDecimal(result);
}

export function useWallet() {
  const [address, setAddress]   = useState(null);
  const [chainId, setChainId]   = useState(null);
  const [balances, setBalances] = useState({});
  const [connecting, setConnecting] = useState(false);
  const [error, setError]       = useState(null);
  const [balancesError, setBalancesError] = useState(null);

  const provider = typeof window !== 'undefined' ? window.ethereum : null;

  const fetchBalances = useCallback(async (addr) => {
    if (!provider || !addr) return;
    try {
      // Native MNT
      const nativeHex = await provider.request({
        method: 'eth_getBalance',
        params: [addr, 'latest'],
      });
      const mntRaw = hexToDecimal(nativeHex);

      // ERC-20 tokens
      const results = await Promise.allSettled(
        Object.entries(TOKEN_CONTRACTS).map(async ([sym, { address: tAddr, decimals }]) => {
          const raw = await erc20Balance(provider, tAddr, addr);
          return { sym, raw, decimals };
        })
      );

      const next = { MNT: formatUnits(mntRaw, 18) };
      for (const r of results) {
        if (r.status === 'fulfilled') {
          const { sym, raw, decimals } = r.value;
          next[sym] = formatUnits(raw, decimals);
        }
      }
      setBalances(next);
      setBalancesError(null);
    } catch (e) {
      console.warn('[useWallet] fetchBalances error', e);
      setBalancesError(e.message || 'Помилка завантаження балансів');
    }
  }, [provider]);

  const switchToMantle = useCallback(async () => {
    if (!provider) return;
    try {
      await provider.request({
        method: 'wallet_switchEthereumChain',
        params: [{ chainId: MANTLE_CHAIN_ID }],
      });
    } catch (switchErr) {
      // chain not added yet
      if (switchErr.code === 4902) {
        await provider.request({
          method: 'wallet_addEthereumChain',
          params: [MANTLE_NETWORK],
        });
      } else {
        throw switchErr;
      }
    }
  }, [provider]);

  const connect = useCallback(async () => {
    if (!provider) {
      setError('MetaMask не знайдено. Встанови розширення.');
      return;
    }
    setConnecting(true);
    setError(null);
    try {
      // Connect account first — always works regardless of current network
      const accounts = await provider.request({ method: 'eth_requestAccounts' });
      const addr = accounts[0];
      setAddress(addr);

      const cid = await provider.request({ method: 'eth_chainId' });
      setChainId(cid);

      // Try to switch to Mantle — non-fatal; wrong-network UI handles it
      try {
        await switchToMantle();
        const newCid = await provider.request({ method: 'eth_chainId' });
        setChainId(newCid);
      } catch (switchErr) {
        console.warn('[useWallet] network switch failed:', switchErr.message);
      }

      await fetchBalances(addr);
    } catch (e) {
      setError(e.message || 'Помилка підключення');
    } finally {
      setConnecting(false);
    }
  }, [provider, switchToMantle, fetchBalances]);

  const disconnect = useCallback(() => {
    setAddress(null);
    setBalances({});
    setChainId(null);
  }, []);

  const switchNetwork = useCallback(async () => {
    try {
      await switchToMantle();
      const cid = await provider.request({ method: 'eth_chainId' });
      setChainId(cid);
      if (address) fetchBalances(address);
    } catch (e) {
      setError(e.message || 'Не вдалось переключити мережу');
    }
  }, [provider, switchToMantle, address, fetchBalances]);

  const refreshBalances = useCallback(() => fetchBalances(address), [fetchBalances, address]);

  // Listen for account/chain changes
  useEffect(() => {
    if (!provider) return;

    const onAccounts = (accounts) => {
      if (accounts.length === 0) disconnect();
      else {
        setAddress(accounts[0]);
        fetchBalances(accounts[0]);
      }
    };
    const onChain = (cid) => setChainId(cid);

    provider.on('accountsChanged', onAccounts);
    provider.on('chainChanged', onChain);
    return () => {
      provider.removeListener('accountsChanged', onAccounts);
      provider.removeListener('chainChanged', onChain);
    };
  }, [provider, disconnect, fetchBalances]);

  // Auto-connect if already permitted
  useEffect(() => {
    if (!provider) return;
    provider.request({ method: 'eth_accounts' }).then((accounts) => {
      if (accounts.length > 0) {
        setAddress(accounts[0]);
        provider.request({ method: 'eth_chainId' }).then(setChainId);
        fetchBalances(accounts[0]);
      }
    });
  }, [provider, fetchBalances]);

  const isMantle = chainId === MANTLE_CHAIN_ID;

  return {
    address,
    chainId,
    isMantle,
    balances,
    balancesError,
    connecting,
    error,
    connect,
    disconnect,
    switchNetwork,
    refreshBalances,
  };
}
