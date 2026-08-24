package ext4

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/diskfs/go-diskfs/backend/file"
	"github.com/google/uuid"
)

// TestCreateInodesPerGroupDivisibleByInodesPerBlock guards against MVM-545:
// go-diskfs used to round inodes_per_group to a multiple of 8 instead of
// inodes_per_block (16 for 4 KiB blocks / 256-byte inodes). At 650 MiB that
// yielded 6936 (% 16 == 8) and Linux 6.1 ext4lazyinit rejected group 0.
func TestCreateInodesPerGroupDivisibleByInodesPerBlock(t *testing.T) {
	const size int64 = 650 * 1024 * 1024

	outfile, f := testCreateEmptyFile(t, size)
	defer f.Close()

	fsuuid := uuid.MustParse("6d696372-6f76-4d00-b007-66735f763100")
	fs, err := Create(file.New(f, false), size, 0, 512, &Params{
		VolumeName:      "rootfs",
		UUID:            &fsuuid,
		SectorsPerBlock: 8,
		Features:        []FeatureOpt{WithFeatureReservedGDTBlocksForExpansion(false)},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if fs == nil {
		t.Fatal("expected non-nil filesystem")
	}

	raw, err := os.ReadFile(outfile)
	if err != nil {
		t.Fatalf("read image: %v", err)
	}
	const superblockOff = 1024
	const inodesPerGroupOff = 0x28
	if len(raw) < superblockOff+inodesPerGroupOff+4 {
		t.Fatalf("image too small for superblock")
	}
	ipg := binary.LittleEndian.Uint32(raw[superblockOff+inodesPerGroupOff:])
	const inodesPerBlock = 4096 / 256
	if ipg%inodesPerBlock != 0 {
		t.Fatalf("inodes_per_group=%d not divisible by inodes_per_block=%d", ipg, inodesPerBlock)
	}

	// Regression pin: old rounding produced 6936 at this size.
	if ipg == 6936 {
		t.Fatalf("inodes_per_group still 6936 (illegal geometry for Linux 6.1)")
	}
}
