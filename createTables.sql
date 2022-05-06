CREATE TABLE `countries`(
    `id` INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    `name` VARCHAR(255) NOT NULL,
    `url` VARCHAR(255) NOT NULL
);
CREATE TABLE `swift_codes`(
    `id` INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    `country_id` INT UNSIGNED UNIQUE NOT NULL,
    `swift_bic` CHAR(255) NOT NULL,
    `bank_institution` VARCHAR(255) NOT NULL,
    `branch_name` VARCHAR(255) NOT NULL,
    `address` TEXT NOT NULL,
    `city_name` VARCHAR(255) NOT NULL,
    `postcode` INT UNSIGNED NOT NULL,
    `connection` TINYINT UNSIGNED NOT NULL
);
CREATE TABLE `logs` (
    `id` INT UNSIGNED NOT NULL PRIMARY KEY
    `place` VARCHAR(255) NOT NULL
    `message` TEXT NOT NULL
)
ALTER TABLE
    `swift_codes` ADD CONSTRAINT `swift_codes_country_id_foreign` FOREIGN KEY(`country_id`) REFERENCES `countries`(`id`);
