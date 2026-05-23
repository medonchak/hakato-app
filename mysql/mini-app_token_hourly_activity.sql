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
-- Table structure for table `token_hourly_activity`
--

DROP TABLE IF EXISTS `token_hourly_activity`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `token_hourly_activity` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `chain_id` bigint NOT NULL,
  `token` varchar(42) NOT NULL,
  `hour_ts` bigint NOT NULL,
  `transfer_count` bigint NOT NULL DEFAULT '0',
  `total_volume_raw` decimal(65,0) NOT NULL DEFAULT '0',
  `exchange_in_raw` decimal(65,0) NOT NULL DEFAULT '0',
  `exchange_in_usd` decimal(30,10) DEFAULT NULL,
  `exchange_out_raw` decimal(65,0) NOT NULL DEFAULT '0',
  `max_transfer_raw` decimal(65,0) NOT NULL DEFAULT '0',
  `max_transfer_usd` decimal(30,10) DEFAULT NULL,
  `exchange_transfer_count` bigint NOT NULL DEFAULT '0',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `total_volume_usd` decimal(30,10) DEFAULT NULL,
  `whale_transfer_count` bigint DEFAULT '0',
  `max_pct_of_supply` decimal(10,8) DEFAULT NULL,
  `exchange_out_usd` decimal(30,10) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_chain_token_hour` (`chain_id`,`token`,`hour_ts`),
  KEY `idx_chain_hour` (`chain_id`,`hour_ts`),
  KEY `idx_token` (`token`),
  KEY `idx_hourly_token_time` (`chain_id`,`token`,`hour_ts`),
  KEY `idx_tha_hour` (`hour_ts`),
  KEY `idx_tha_chain_hour` (`chain_id`,`hour_ts`),
  KEY `idx_tha_token_hour` (`token`,`hour_ts`)
) ENGINE=InnoDB AUTO_INCREMENT=72367 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
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
