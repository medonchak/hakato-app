-- MySQL dump 10.13  Distrib 8.0.33, for Win64 (x86_64)
--
-- Host: localhost    Database: mini-app
-- ------------------------------------------------------
-- Server version	8.0.33

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!50503 SET NAMES utf8 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `portfolio_notifications`
--

DROP TABLE IF EXISTS `portfolio_notifications`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `portfolio_notifications` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `portfolio_id` bigint NOT NULL,
  `chain_id` bigint NOT NULL,
  `token` varchar(42) NOT NULL,
  `tx_hash` varchar(66) NOT NULL,
  `block_time` datetime NOT NULL,
  `direction` varchar(8) NOT NULL,
  `amount_raw` decimal(65,0) NOT NULL,
  `exchange_name` varchar(128) DEFAULT NULL,
  `rule_hash` varchar(256) DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `notif_type` enum('EVENT','DAILY_REPORT') NOT NULL DEFAULT 'EVENT',
  `day_ts` bigint DEFAULT NULL,
  `payload_text` text,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_daily_report` (`portfolio_id`,`chain_id`,`token`,`day_ts`,`notif_type`),
  KEY `idx_portfolio_time` (`portfolio_id`,`created_at`),
  KEY `idx_token_time` (`token`,`created_at`),
  KEY `idx_tx` (`tx_hash`),
  KEY `idx_notif_portfolio_time` (`portfolio_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2026-02-23 22:09:56
