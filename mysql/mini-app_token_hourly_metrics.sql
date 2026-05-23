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
-- Table structure for table `token_hourly_metrics`
--

DROP TABLE IF EXISTS `token_hourly_metrics`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `token_hourly_metrics` (
  `chain_id` bigint NOT NULL,
  `token` char(42) NOT NULL,
  `hour_ts` bigint NOT NULL,
  `transfers` bigint NOT NULL,
  `unique_senders` bigint NOT NULL,
  `unique_receivers` bigint NOT NULL,
  `unique_addresses` bigint NOT NULL,
  `p50_raw` decimal(65,0) DEFAULT NULL,
  `p95_raw` decimal(65,0) DEFAULT NULL,
  `p99_raw` decimal(65,0) DEFAULT NULL,
  `p50_usd` double DEFAULT NULL,
  `p95_usd` double DEFAULT NULL,
  `p99_usd` double DEFAULT NULL,
  `top1_addr_share` double DEFAULT NULL,
  `top3_addr_share` double DEFAULT NULL,
  `top5_addr_share` double DEFAULT NULL,
  `exchange_share` double DEFAULT NULL,
  `net_exchange_usd` double DEFAULT NULL,
  `usd_lt_100` bigint NOT NULL,
  `usd_100_1k` bigint NOT NULL,
  `usd_1k_10k` bigint NOT NULL,
  `usd_10k_100k` bigint NOT NULL,
  `usd_gt_100k` bigint NOT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`chain_id`,`token`,`hour_ts`),
  KEY `idx_hour` (`hour_ts`),
  KEY `idx_thm_hour` (`hour_ts`),
  KEY `idx_thm_chain_hour` (`chain_id`,`hour_ts`),
  KEY `idx_thm_token_hour` (`token`,`hour_ts`)
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

-- Dump completed on 2026-02-23 22:09:55
