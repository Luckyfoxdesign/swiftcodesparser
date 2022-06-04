CREATE TABLE `countries`(
    `id` INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    `name` VARCHAR(255) NOT NULL,
    `url` VARCHAR(255) NOT NULL
);
CREATE TABLE `swift_codes`(
    `id` INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    `country_id` INT UNSIGNED NOT NULL,
    `swift_bic` VARCHAR(255) NOT NULL,
    `bank_institution` VARCHAR(255) NOT NULL,
    `branch_name` VARCHAR(255) NOT NULL,
    `address` TEXT,
    `city_name` VARCHAR(255) NOT NULL,
    `postcode` INT UNSIGNED NOT NULL,
    `connection` TINYINT UNSIGNED NOT NULL
);
CREATE TABLE `logs` (
    `id` INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    `place` VARCHAR(255) NOT NULL,
    `message` TEXT NOT NULL
);
CREATE TABLE `progress_temp` (
    `id` INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    `name` VARCHAR(255) NOT NULL,
    `url` VARCHAR(255) NOT NULL,
    `pages_total` TINYINT UNSIGNED NOT NULL DEFAULT 0,

    -- Номера страниц которые не удалось спарсить. 
    -- Page numbers with the comma separator. Pages that didn't parsed
    -- Field data example: 1,33,11,...N
    `pages_numbers` TEXT NOT NULL DEFAULT 0,

    -- 0 - not started
    -- 1 - finished
    -- 2 - partially finished
    `status` TINYINT UNSIGNED NOT NULL DEFAULT 0
);