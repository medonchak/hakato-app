// dashboardStyles.js

export const palette = {
  // Теплий, затишний, сучасний фон і акценти
  bgMain: 'linear-gradient(135deg, #f7eee6 0%, #f2ede7 35%, #e9f3f7 100%)',
  card: 'rgba(252,249,242,0.98)',        // latte/powder
  cardBorder: '1.5px solid #ebdfc8',
  accentBlue: '#47a6c7',                 // спокійний блакитний
  accentPink: '#ee7d9f',                 // м’який рожевий
  accentMint: '#96dec9',                 // пастельна м’ята
  white: '#23272f',                      // темний текст
  gray: '#78818d',                       // спокійний сірий
};

export const warmStyles = {
  container: {
    pb: 10,
    minHeight: '100vh',
    pt: { xs: 2, md: 5 },
    background: palette.bgMain,
    maxWidth: 1200,
    mx: 'auto',
    px: { xs: 2, md: 4 },
  },
  header: {
    textAlign: 'center',
    mb: 3,
  },
  title: {
    fontWeight: 800,
    color: palette.white,
    letterSpacing: 2,
    textShadow: '0 2px 14px #e9f3f755, 0 1px 0 #ebdfc8',
  },
  subtitle: {
    color: palette.accentPink,
    fontWeight: 600,
    fontSize: 16,
    letterSpacing: 1,
    opacity: 0.8,
  },
  chartPaper: {
    p: 2.5,
    background: palette.card, // теплий latte фон!
    borderRadius: 3,
    boxShadow: '0 2px 12px #96dec922',
  },
  chartBoxBg: {
    height: 100,
    background: 'rgba(239,238,232,0.88)', // дуже м’який молочний фон (для світлої теми)
    mt: 2,
    borderRadius: 3,
  },
  avatarWrap: {
    display: 'flex',
    justifyContent: 'flex-end',
    alignItems: 'center',
    gap: 1.5,
    mt: 1,
    mb: 2,
  },
  avatarImg: {
    border: `2.5px solid ${palette.accentBlue}`,
    boxShadow: '0 0 7px #47a6c777',
    width: 44,
    height: 44,
  },
  menuBtn: {
    boxShadow: '0 0 9px #47a6c744',
    border: `2px solid ${palette.accentBlue}`,
    bgcolor: 'rgba(224,230,237,0.54)',
    p: 0.5,
    ml: 1,
  },
  menuPaper: {
    borderRadius: 3,
    bgcolor: 'rgba(252,249,242,0.98)',
    boxShadow: '0 0 24px #47a6c733',
    p: 0,
  },
  menuInner: {
    px: 2.5,
    py: 2,
    width: 320,
  },
  searchBox: {
    background: palette.card,
    borderRadius: 3,
    boxShadow: '0 0 9px #96dec933',
    input: { color: palette.white }
  },
  filtersWrap: {
    display: 'flex',
    justifyContent: 'space-between',
    my: 3,
    gap: 2,
    flexWrap: 'wrap',
  },
  formControl: {
    minWidth: 160,
    bgcolor: palette.card,
    borderRadius: 2,
    boxShadow: '0 0 7px #47a6c722',
  },
  inputLabel: {
    color: palette.gray,
    opacity: 0.78,
    fontWeight: 500,
    fontSize: 13,
    mb: 0.5,
  },
  select: {
    color: palette.accentBlue,
    bgcolor: 'rgba(235,239,242,0.89)',
    borderRadius: 2,
    fontWeight: 600,
  },
  statsGrid: {
    mt: 2,
    mb: 2,
  },
  statsPaper: {
    p: 2,
    textAlign: 'center',
    background: palette.card,
    boxShadow: '0 1.5px 8px #47a6c714',
    border: palette.cardBorder,
    borderRadius: 2,
  },
  statsLabel: {
    color: palette.accentPink,
    fontWeight: 700,
    letterSpacing: 1,
    fontSize: 15,
    mb: 0.5,
  },
  statsValue: {
    color: palette.accentBlue,
    fontWeight: 800,
    fontSize: 28,
    letterSpacing: 1,
  },
  chartPaper: {
    p: 2.5,
    background: 'rgba(239,238,232,0.97)',
    borderRadius: 3,
    boxShadow: '0 2px 12px #96dec922',
  },
  chartTitle: {
    color: palette.accentBlue,
    fontWeight: 700,
    mb: 2,
  },
  topPaper: {
    p: 2,
    borderRadius: 3,
    background: palette.card,
    boxShadow: '0 0 7px #47a6c71a',
  },
  divider: {
    my: 1,
    background: `linear-gradient(90deg,${palette.accentBlue},${palette.accentPink})`,
    opacity: 0.55,
    height: 3,
    borderRadius: 99,
    },
    adress: {
    color: '#47a6c7',       // темно-сірий, м'який
    fontWeight: 600,
    letterSpacing: 0.3,
    mb: 0.4,
    },
  tx: {
    color: '#47a6c7',       // м’який спокійний блакитний
    fontWeight: 700,
    marginLeft: 6,
  },
  footer: {
    textAlign: "center",
    color: palette.accentPink,
    opacity: 0.45,
    mt: 8,
    letterSpacing: 1,
  },
  headerPanel: {
    background: 'linear-gradient(90deg,#e3ecee,#f5e5df 90%)',
    color: '#23272f',
    boxShadow: '0 2px 12px #d6e1e440',
  },
    // === Alert Form ===
  alertFormWrap: {
    maxWidth: 1000,
    mx: 'auto',
    background: palette.card,
    border: palette.cardBorder,
    borderRadius: 3,
    boxShadow: '0 2px 12px #96dec922',
    p: { xs: 2, md: 3 },
    mt: 3,
  },
  alertFormTitle: {
    fontWeight: 800,
    color: palette.accentBlue,
    letterSpacing: 1,
    mb: 2,
    textShadow: '0 1px 8px #96dec922',
  },
  alertFieldset: {
    border: palette.cardBorder,
    borderRadius: 2,
    p: 2,
    mb: 2,
    background: 'rgba(239,238,232,0.58)',
  },
  alertLegend: {
    px: 1,
    color: palette.accentPink,
    fontWeight: 700,
    letterSpacing: 0.6,
  },
  row2: {
    display: 'grid',
    gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' },
    gap: 1,
  },
  row3: {
    display: 'grid',
    gridTemplateColumns: { xs: '1fr', md: '1fr 1fr 1fr' },
    gap: 1,
  },
  input: {
    bgcolor: 'rgba(235,239,242,0.89)',
    borderRadius: 2,
    '& .MuiInputBase-input': { color: palette.white, fontWeight: 500 },
    '& .MuiOutlinedInput-notchedOutline': { borderColor: '#e6ddd0' },
    '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: palette.accentBlue },
    '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: palette.accentBlue },
  },
  selectInput: {
    bgcolor: 'rgba(235,239,242,0.89)',
    borderRadius: 2,
    '& .MuiSelect-select': { color: palette.accentBlue, fontWeight: 600 },
    '& .MuiOutlinedInput-notchedOutline': { borderColor: '#e6ddd0' },
    '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: palette.accentBlue },
    '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: palette.accentBlue },
  },
  cardItem: {
    p: 1.5,
    mt: 1,
    mb: 1,
    borderRadius: 2,
    background: palette.card,
    border: palette.cardBorder,
    boxShadow: '0 1px 8px #47a6c714',
  },
  btn: {
    mt: 1,
    textTransform: 'none',
    fontWeight: 700,
    borderRadius: 2,
    border: `2px solid ${palette.accentBlue}`,
    color: palette.accentBlue,
    background: 'rgba(224,230,237,0.54)',
    boxShadow: '0 0 9px #47a6c744',
    '&:hover': { background: 'rgba(224,230,237,0.72)' },
  },
  btnDanger: {
    mt: 1,
    textTransform: 'none',
    fontWeight: 700,
    borderRadius: 2,
    border: `2px solid ${palette.accentPink}`,
    color: palette.accentPink,
    background: 'rgba(252, 235, 240, 0.55)',
    boxShadow: '0 0 9px #ee7d9f55',
    '&:hover': { background: 'rgba(252, 235, 240, 0.75)' },
  },
  preBox: {
    mt: 2,
    p: 2,
    borderRadius: 2,
    background: 'rgba(239,238,232,0.97)',
    color: palette.white,
    border: palette.cardBorder,
    boxShadow: 'inset 0 1px 8px #96dec922',
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace',
    fontSize: 13,
    whiteSpace: 'pre-wrap',
  },
  portfolioWrap: {
    maxWidth: 700,
    mx: 'auto',
    mt: 3,
    p: 3,
    borderRadius: 3,
    background: 'rgba(252,249,242,0.98)',
    boxShadow: '0 2px 12px #96dec922',
  },
  portfolioHeader: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    mb: 2,
  },
  portfolioTitle: {
    fontWeight: 800,
    color: palette.accentBlue,
    fontSize: 22,
    letterSpacing: 1,
  },
  portfolioForm: {
    display: 'flex',
    gap: 1.5,
    mb: 2,
    flexWrap: 'wrap',
  },
  portfolioInput: {
    flex: 1,
    bgcolor: 'rgba(235,239,242,0.89)',
    borderRadius: 2,
    px: 1.5,
    py: 0.8,
    border: `1.5px solid ${palette.accentBlue}`,
    '&:focus': { outline: 'none', borderColor: palette.accentMint },
    color: palette.white,
    fontWeight: 500,
  },
  portfolioBtn: {
    textTransform: 'none',
    fontWeight: 700,
    borderRadius: 2,
    border: `2px solid ${palette.accentBlue}`,
    color: palette.accentBlue,
    background: 'rgba(224,230,237,0.54)',
    boxShadow: '0 0 9px #47a6c744',
    '&:hover': { background: 'rgba(224,230,237,0.72)' },
    px: 2,
  },
  portfolioTable: {
    width: '100%',
    borderCollapse: 'collapse',
    mb: 2,
    th: {
      textAlign: 'left',
      borderBottom: `1px solid ${palette.accentBlue}`,
      color: palette.accentBlue,
      fontWeight: 600,
      py: 1,
      px: 1.5,
    },
    td: {
      py: 1,
      px: 1.5,
      color: palette.white,
    },
    trOdd: {
      background: 'rgba(239,238,232,0.35)',
    },
  },
  portfolioTotal: {
    textAlign: 'right',
    fontWeight: 800,
    fontSize: 18,
    color: palette.accentMint,
  }
};
export const portfolio = {
  wrap: {
    ...warmStyles.chartPaper,
    mt: 3,
    p: 3,
  },
  header: {
    ...warmStyles.title,
    fontSize: 22,
    mb: 2,
  },
  form: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: 12,
    mb: 2,
  },
  inputSymbol: {
    ...warmStyles.input,
    flex: 1,
  },
  inputAmount: {
    ...warmStyles.input,
    width: 120,
  },
  addBtn: {
    ...warmStyles.btn,
  },
  card: {
    ...warmStyles.cardItem,
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    mb: 1.5,
  },
  cardLeft: {
    display: 'flex',
    flexDirection: 'column',
    gap: 2,
  },
  symbol: {
    ...warmStyles.subtitle,
    fontWeight: 700,
    fontSize: 16,
  },
  amount: {
    color: warmStyles.adress.color,
    fontWeight: 500,
  },
  price: {
    color: warmStyles.accentBlue,
    fontWeight: 600,
  },
  value: {
    color: warmStyles.accentMint,
    fontWeight: 700,
  },
  removeBtn: {
    ...warmStyles.btnDanger,
    height: 34,
    alignSelf: 'flex-start',
  },
  total: {
    ...warmStyles.statsValue,
    mt: 2,
  },
};
// analyticsStyles як раніше, або адаптуй за бажанням

// dashboardStyles.js (додай до warmStyles)


