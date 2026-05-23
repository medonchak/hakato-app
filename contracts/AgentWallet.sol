// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface IERC20 {
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
    function approve(address spender, uint256 amount) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
}

/// @notice Merchant Moe router interface (Mantle DEX).
interface IMerchantMoeRouter {
    struct ExactInputSingleParams {
        address tokenIn;
        address tokenOut;
        uint24  fee;
        address recipient;
        uint256 deadline;
        uint256 amountIn;
        uint256 amountOutMinimum;
        uint160 sqrtPriceLimitX96;
    }
    function exactInputSingle(ExactInputSingleParams calldata params)
        external payable returns (uint256 amountOut);
}

/// @notice Agent-controlled wallet for executing swaps on Mantle via Merchant Moe.
/// Safety limits: maxTradeUSD, dailyLimit, cooldown between trades.
contract AgentWallet {
    address public owner;
    address public agent;    // authorized Go-backend key

    // Merchant Moe router on Mantle mainnet
    address public immutable router;

    // Safety parameters (in USD, scaled ×1e8 to match Chainlink)
    uint256 public maxTradeUSD   = 5 * 1e8;    // $5
    uint256 public dailyLimitUSD = 50 * 1e8;   // $50
    uint256 public cooldownSec   = 300;         // 5 minutes between trades

    // State
    uint256 public dailySpentUSD;
    uint256 public lastTradeDay;
    uint256 public lastTradeTs;

    struct TradeRecord {
        address tokenIn;
        address tokenOut;
        uint256 amountIn;
        uint256 amountOut;
        uint256 timestamp;
    }

    TradeRecord[] public trades;

    event Swapped(
        uint256 indexed tradeId,
        address indexed tokenIn,
        address indexed tokenOut,
        uint256 amountIn,
        uint256 amountOut,
        uint256 tradeUSD
    );

    event Deposited(address indexed token, uint256 amount);
    event Withdrawn(address indexed token, uint256 amount, address to);

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    modifier onlyAgent() {
        require(msg.sender == agent || msg.sender == owner, "not agent");
        _;
    }

    constructor(address _router, address _agent) {
        owner  = msg.sender;
        agent  = _agent;
        router = _router;
    }

    receive() external payable {}

    // ─── Agent actions ───────────────────────────────────────────────

    function swap(
        address tokenIn,
        address tokenOut,
        uint24  fee,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 tradeUSD       // caller-supplied USD value (scaled ×1e8) for limits check
    ) external onlyAgent returns (uint256 amountOut) {
        _checkLimits(tradeUSD);

        IERC20(tokenIn).approve(router, amountIn);

        amountOut = IMerchantMoeRouter(router).exactInputSingle(
            IMerchantMoeRouter.ExactInputSingleParams({
                tokenIn:            tokenIn,
                tokenOut:           tokenOut,
                fee:                fee,
                recipient:          address(this),
                deadline:           block.timestamp + 60,
                amountIn:           amountIn,
                amountOutMinimum:   amountOutMin,
                sqrtPriceLimitX96:  0
            })
        );

        uint256 day = block.timestamp / 1 days;
        if (day != lastTradeDay) {
            dailySpentUSD = 0;
            lastTradeDay  = day;
        }
        dailySpentUSD += tradeUSD;
        lastTradeTs    = block.timestamp;

        uint256 id = trades.length;
        trades.push(TradeRecord({
            tokenIn:   tokenIn,
            tokenOut:  tokenOut,
            amountIn:  amountIn,
            amountOut: amountOut,
            timestamp: block.timestamp
        }));

        emit Swapped(id, tokenIn, tokenOut, amountIn, amountOut, tradeUSD);
    }

    // ─── Owner management ────────────────────────────────────────────

    function deposit(address token, uint256 amount) external onlyOwner {
        IERC20(token).transferFrom(msg.sender, address(this), amount);
        emit Deposited(token, amount);
    }

    function withdraw(address token, uint256 amount, address to) external onlyOwner {
        IERC20(token).transfer(to, amount);
        emit Withdrawn(token, amount, to);
    }

    function withdrawETH(address payable to) external onlyOwner {
        uint256 bal = address(this).balance;
        (bool ok,) = to.call{value: bal}("");
        require(ok, "ETH transfer failed");
    }

    function setAgent(address _agent) external onlyOwner {
        agent = _agent;
    }

    function setLimits(uint256 _maxTradeUSD, uint256 _dailyLimitUSD, uint256 _cooldownSec) external onlyOwner {
        maxTradeUSD   = _maxTradeUSD;
        dailyLimitUSD = _dailyLimitUSD;
        cooldownSec   = _cooldownSec;
    }

    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "zero address");
        owner = newOwner;
    }

    // ─── Views ───────────────────────────────────────────────────────

    function totalTrades() external view returns (uint256) {
        return trades.length;
    }

    function getTrade(uint256 id) external view returns (TradeRecord memory) {
        return trades[id];
    }

    function tokenBalance(address token) external view returns (uint256) {
        return IERC20(token).balanceOf(address(this));
    }

    // ─── Internal ────────────────────────────────────────────────────

    function _checkLimits(uint256 tradeUSD) internal view {
        require(tradeUSD <= maxTradeUSD, "exceeds maxTradeUSD");

        uint256 day = block.timestamp / 1 days;
        uint256 spent = (day == lastTradeDay) ? dailySpentUSD : 0;
        require(spent + tradeUSD <= dailyLimitUSD, "daily limit reached");

        require(block.timestamp >= lastTradeTs + cooldownSec, "cooldown active");
    }
}
