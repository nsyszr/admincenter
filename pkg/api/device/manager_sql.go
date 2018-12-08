package device

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

func NewSQLManager(db *sqlx.DB) *SQLManager {
	return &SQLManager{
		DB: db,
	}
}

type SQLManager struct {
	DB *sqlx.DB
}

type sqlData struct {
	PK                   int       `db:"pk"`
	ID                   string    `db:"device_id"`
	DeviceType           string    `db:"device_type"`
	Name                 string    `db:"device_name"`
	Location             string    `db:"device_location"`
	AssetTag             string    `db:"asset_tag"`
	Group                string    `db:"device_group"`
	HardwareModel        string    `db:"hardware_model"`
	HardwareRevision     string    `db:"hardware_revision"`
	HardwareSerialNumber string    `db:"hardware_serial_number"`
	FirmwareName         string    `db:"firmware_name"`
	FirmwareVersion      string    `db:"firmware_version"`
	UpdatedAt            time.Time `db:"updated_at"`
	CreatedAt            time.Time `db:"created_at"`
}

var sqlParams = []string{
	"device_id",
	"device_type",
	"device_name",
	"device_location",
	"asset_tag",
	"device_group",
	"hardware_model",
	"hardware_revision",
	"hardware_serial_number",
	"firmware_name",
	"firmware_version",
	"updated_at",
	"created_at",
}

func sqlDataFromDTO(dto *Device) (*sqlData, error) {
	var createdAt, updatedAt = dto.CreatedAt, dto.UpdatedAt

	if dto.CreatedAt.IsZero() {
		createdAt = time.Now()
	}

	if dto.UpdatedAt.IsZero() {
		updatedAt = time.Now()
	}

	return &sqlData{
		ID:                   dto.GetID(),
		DeviceType:           dto.DeviceType,
		Name:                 dto.Name,
		Location:             dto.Location,
		AssetTag:             dto.AssetTag,
		Group:                dto.Group,
		HardwareModel:        dto.HardwareModel,
		HardwareRevision:     dto.HardwareRevision,
		HardwareSerialNumber: dto.HardwareSerialNumber,
		FirmwareName:         dto.FirmwareName,
		FirmwareVersion:      dto.FirmwareVersion,
		CreatedAt:            createdAt.Round(time.Second),
		UpdatedAt:            updatedAt.Round(time.Second),
	}, nil
}

func (d *sqlData) ToDTO() (*Device, error) {
	dto := &Device{
		DeviceID:             d.ID,
		DeviceType:           d.DeviceType,
		Name:                 d.Name,
		Location:             d.Location,
		AssetTag:             d.AssetTag,
		Group:                d.Group,
		HardwareModel:        d.HardwareModel,
		HardwareRevision:     d.HardwareRevision,
		HardwareSerialNumber: d.HardwareSerialNumber,
		FirmwareName:         d.FirmwareName,
		FirmwareVersion:      d.FirmwareVersion,
		CreatedAt:            d.CreatedAt,
		UpdatedAt:            d.UpdatedAt,
	}

	return dto, nil
}

func (m *SQLManager) GetDevices(ctx context.Context, limit, offset int) (dtos map[string]Device, err error) {
	data := make([]sqlData, 0)
	dtos = make(map[string]Device)

	if err := m.DB.SelectContext(ctx, &data, m.DB.Rebind("SELECT * FROM devices ORDER BY device_id LIMIT ? OFFSET ?"), limit, offset); err != nil {
		return nil, err
	}

	for _, d := range data {
		dto, err := d.ToDTO()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		dtos[d.ID] = *dto
	}
	return dtos, nil
}

func (m *SQLManager) GetConcreteDevice(ctx context.Context, id string) (*Device, error) {
	var d sqlData
	if err := m.DB.GetContext(ctx, &d, m.DB.Rebind("SELECT * FROM devices WHERE device_id=?"), id); err != nil {
		return nil, err
	}

	return d.ToDTO()
}

func (m *SQLManager) CreateDevice(ctx context.Context, dto *Device) error {
	data, err := sqlDataFromDTO(dto)
	if err != nil {
		return errors.WithStack(err)
	}

	if _, err := m.DB.NamedExecContext(ctx, fmt.Sprintf(
		"INSERT INTO devices (%s) VALUES (%s)",
		strings.Join(sqlParams, ", "),
		":"+strings.Join(sqlParams, ", :"),
	), data); err != nil {
		return err
	}

	return nil
}

func (m *SQLManager) UpdateDevice(ctx context.Context, dto *Device) error {
	_, err := m.GetConcreteDevice(ctx, dto.GetID())
	if err != nil {
		return errors.WithStack(err)
	}

	data, err := sqlDataFromDTO(dto)
	if err != nil {
		return errors.WithStack(err)
	}

	var query []string
	for _, param := range sqlParams {
		query = append(query, fmt.Sprintf("%s=:%s", param, param))
	}

	if _, err := m.DB.NamedExecContext(ctx, fmt.Sprintf(`UPDATE devices SET %s WHERE device_id=:device_id`, strings.Join(query, ", ")), data); err != nil {
		return err
	}

	return nil
}

func (m *SQLManager) DeleteDevice(ctx context.Context, id string) error {
	if _, err := m.DB.ExecContext(ctx, m.DB.Rebind(`DELETE FROM devices WHERE device_id=?`), id); err != nil {
		return err
	}
	return nil
}
