-- +migrate Up
CREATE TABLE IF NOT EXISTS devices (
    pk                      serial,
	device_id               varchar(255) NOT NULL,
	device_type             text NOT NULL,
	device_name             text NOT NULL,
	device_location         text NOT NULL,
	asset_tag               text NOT NULL,
	device_group            text NOT NULL,
	hardware_model          text NOT NULL,
	hardware_revision       text NOT NULL,
	hardware_serial_number  text NOT NULL,
	firmware_name           text NOT NULL,
	firmware_version        text NOT NULL,
    created_at              timestamp NOT NULL DEFAULT now(),
    updated_at              timestamp NOT NULL DEFAULT now(),
    PRIMARY KEY (pk)
);
CREATE UNIQUE INDEX devices_idx_device_id_uq ON devices (device_id);

-- +migrate Down
DROP INDEX devices_idx_device_id_uq;
DROP TABLE devices;