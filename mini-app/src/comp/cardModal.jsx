import { Modal, Box, Fade, Backdrop } from "@mui/material";
import { palette } from "../dashboardWarmStyles"; // імпорт твоєї палітри

const CardModal = ({ open, onClose, children }) => {
  return (
    <Modal
      open={open}
      onClose={onClose}
      closeAfterTransition
      slots={{ backdrop: Backdrop }}
      slotProps={{
        backdrop: {
          timeout: 200,
          sx: {
            background: "rgba(0,0,0,0.15)",      // легкий напівпрозорий фон
            backdropFilter: "blur(3px)",          // м’який blur
          }
        }
      }}
    >
      <Fade in={open}>
        <Box
          sx={{
            position: "absolute",
            top: "50%",
            left: "50%",
            transform: "translate(-50%, -50%)",

            // 🔥 Теплий фон картки
            bgcolor: palette.card,
            color: palette.white,

            // 🔥 Стиль картки як у warmStyles.cardItem
            borderRadius: 3,
            border: palette.cardBorder,
            boxShadow: "0 4px 18px #47a6c722",

            // 🔥 Адаптив  
            width: "95%",
            maxWidth: "900px",
            maxHeight: "90vh",
            overflowY: "auto",

            p: { xs: 2, md: 3 },

            // 🔥 Плавна анімація
            transition: "0.25s ease"
          }}
        >
          {children}
        </Box>
      </Fade>
    </Modal>
  );
};

export default CardModal;
