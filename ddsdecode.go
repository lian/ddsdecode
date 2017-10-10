package ddsdecode

// source: https://bitbucket.org/SpartanJ/soil2/src/54073b4230378ea9c99f2ec255a6273423f72b3f/src/SOIL2/stbi_DDS_c.h

import (
	"encoding/binary"
	"fmt"
	"io"
)

type Texture struct {
	Header DDS_Header
	Width  int
	Height int
	DXT    int
	Data   []byte
}

type DDS_PixelFormat struct {
	Size         uint32
	Flags        uint32
	FourCC       uint32
	RGBBitCount  uint32
	RBitMask     uint32
	GBitMask     uint32
	BBitMask     uint32
	AlphaBitMask uint32
}

type DDS_Caps struct {
	Caps1    uint32
	Caps2    uint32
	DDSX     uint32
	Reserved uint32
}

type DDS_Header struct {
	Magic             [4]byte
	Size              uint32
	Flags             uint32
	Height            uint32
	Width             uint32
	PitchOrLinearSize uint32
	Depth             uint32
	MipMapCount       uint32
	Reserved1         [11]uint32
	PixelFormat       DDS_PixelFormat // DDPIXELFORMAT
	Caps              DDS_Caps        // DDCAPS
	Reserved2         uint32
}

// the following constants were copied directly off the MSDN website

// The dwFlags member of the original DDSURFACEDESC2 structure can be set to one or more of the following values.
const DDSD_CAPS = 0x00000001
const DDSD_HEIGHT = 0x00000002
const DDSD_WIDTH = 0x00000004
const DDSD_PITCH = 0x00000008
const DDSD_PIXELFORMAT = 0x00001000
const DDSD_MIPMAPCOUNT = 0x00020000
const DDSD_LINEARSIZE = 0x00080000
const DDSD_DEPTH = 0x00800000

// DirectDraw Pixel Format
const DDPF_ALPHAPIXELS = 0x00000001
const DDPF_FOURCC = 0x00000004
const DDPF_RGB = 0x00000040

// The dwCaps1 member of the DDSCAPS2 structure can be set to one or more of the following values.
const DDSCAPS_COMPLEX = 0x00000008
const DDSCAPS_TEXTURE = 0x00001000
const DDSCAPS_MIPMAP = 0x00400000

// The dwCaps2 member of the DDSCAPS2 structure can be set to one or more of the following values.
const DDSCAPS2_CUBEMAP = 0x00000200
const DDSCAPS2_CUBEMAP_POSITIVEX = 0x00000400
const DDSCAPS2_CUBEMAP_NEGATIVEX = 0x00000800
const DDSCAPS2_CUBEMAP_POSITIVEY = 0x00001000
const DDSCAPS2_CUBEMAP_NEGATIVEY = 0x00002000
const DDSCAPS2_CUBEMAP_POSITIVEZ = 0x00004000
const DDSCAPS2_CUBEMAP_NEGATIVEZ = 0x00008000
const DDSCAPS2_VOLUME = 0x00200000

func stbi_convert_bit_range(c, from_bits, to_bits uint32) uint32 {
	b := (1 << (from_bits - 1)) + c*((1<<to_bits)-1)
	return (b + (b >> from_bits)) >> from_bits
}

func stbi_rgb_888_from_565(c uint32) (uint32, uint32, uint32) {
	return stbi_convert_bit_range((c>>11)&31, 5, 8),
		stbi_convert_bit_range((c>>05)&63, 6, 8),
		stbi_convert_bit_range((c>>00)&31, 5, 8)
}

func decode_DXT23_alpha_block(uncompressed, compressed []byte) {
	next_bit := uint32(0)
	//	each alpha value gets 4 bits
	for i := 3; i < 16*4; i += 4 {
		uncompressed[i] = byte(
			stbi_convert_bit_range(
				(uint32(compressed[next_bit>>3])>>(next_bit&7))&15,
				4, 8,
			),
		)
		next_bit += 4
	}
}

func decode_DXT45_alpha_block(uncompressed, compressed []byte) {
	decode_alpha := [8]uint32{}

	//	each alpha value gets 3 bits, and the 1st 2 bytes are the range
	decode_alpha[0] = uint32(compressed[0])
	decode_alpha[1] = uint32(compressed[1])

	if decode_alpha[0] > decode_alpha[1] {
		//	6 step intermediate
		decode_alpha[2] = (6*decode_alpha[0] + 1*decode_alpha[1]) / 7
		decode_alpha[3] = (5*decode_alpha[0] + 2*decode_alpha[1]) / 7
		decode_alpha[4] = (4*decode_alpha[0] + 3*decode_alpha[1]) / 7
		decode_alpha[5] = (3*decode_alpha[0] + 4*decode_alpha[1]) / 7
		decode_alpha[6] = (2*decode_alpha[0] + 5*decode_alpha[1]) / 7
		decode_alpha[7] = (1*decode_alpha[0] + 6*decode_alpha[1]) / 7
	} else {
		//	4 step intermediate, pluss full and none
		decode_alpha[2] = (4*decode_alpha[0] + 1*decode_alpha[1]) / 5
		decode_alpha[3] = (3*decode_alpha[0] + 2*decode_alpha[1]) / 5
		decode_alpha[4] = (2*decode_alpha[0] + 3*decode_alpha[1]) / 5
		decode_alpha[5] = (1*decode_alpha[0] + 4*decode_alpha[1]) / 5
		decode_alpha[6] = 0
		decode_alpha[7] = 255
	}

	next_bit := 8 * 2
	for i := 3; i < 16*4; i += 4 {
		idx := uint(0)
		bit := uint(0)
		bit = uint((compressed[next_bit>>3] >> (uint(next_bit) & 7)) & 1)
		idx += bit << 0
		next_bit += 1
		bit = uint((compressed[next_bit>>3] >> (uint(next_bit) & 7)) & 1)
		idx += bit << 1
		next_bit += 1
		bit = uint((compressed[next_bit>>3] >> (uint(next_bit) & 7)) & 1)
		idx += bit << 2
		next_bit += 1
		uncompressed[i] = byte(decode_alpha[idx&7])
	}
}

func decode_DXT_color_block(uncompressed, compressed []byte) {
	var r, g, b, c0, c1 uint32
	decode_colors := [4 * 3]uint32{}

	//	find the 2 primary colors
	c0 = uint32(compressed[0]) + (uint32(compressed[1]) << 8)
	c1 = uint32(compressed[2]) + (uint32(compressed[3]) << 8)

	r, g, b = stbi_rgb_888_from_565(c0)
	decode_colors[0] = r
	decode_colors[1] = g
	decode_colors[2] = b
	r, g, b = stbi_rgb_888_from_565(c1)
	decode_colors[3] = r
	decode_colors[4] = g
	decode_colors[5] = b

	//	Like DXT1, but no choicees:	no alpha, 2 interpolated colors
	decode_colors[6] = (2*decode_colors[0] + decode_colors[3]) / 3
	decode_colors[7] = (2*decode_colors[1] + decode_colors[4]) / 3
	decode_colors[8] = (2*decode_colors[2] + decode_colors[5]) / 3
	decode_colors[9] = (decode_colors[0] + 2*decode_colors[3]) / 3
	decode_colors[10] = (decode_colors[1] + 2*decode_colors[4]) / 3
	decode_colors[11] = (decode_colors[2] + 2*decode_colors[5]) / 3

	//	decode the block
	next_bit := uint32(4 * 8)
	for i := 0; i < 16*4; i += 4 {
		idx := ((compressed[next_bit>>3] >> (next_bit & 7)) & 3) * 3
		next_bit += 2
		uncompressed[i+0] = byte(decode_colors[idx+0])
		uncompressed[i+1] = byte(decode_colors[idx+1])
		uncompressed[i+2] = byte(decode_colors[idx+2])
	}
}

func decode_DXT1_block(uncompressed, compressed []byte) {
	var r, g, b, c0, c1 uint32
	decode_colors := [4 * 4]uint32{}

	//	find the 2 primary colors
	c0 = uint32(compressed[0]) + (uint32(compressed[1]) << 8)
	c1 = uint32(compressed[2]) + (uint32(compressed[3]) << 8)

	r, g, b = stbi_rgb_888_from_565(c0)
	decode_colors[0] = r
	decode_colors[1] = g
	decode_colors[2] = b
	decode_colors[3] = 255
	r, g, b = stbi_rgb_888_from_565(c1)
	decode_colors[4] = r
	decode_colors[5] = g
	decode_colors[6] = b
	decode_colors[7] = 255

	if c0 > c1 {
		//	no alpha, 2 interpolated colors
		decode_colors[8] = (2*decode_colors[0] + decode_colors[4]) / 3
		decode_colors[9] = (2*decode_colors[1] + decode_colors[5]) / 3
		decode_colors[10] = (2*decode_colors[2] + decode_colors[6]) / 3
		decode_colors[11] = 255
		decode_colors[12] = (decode_colors[0] + 2*decode_colors[4]) / 3
		decode_colors[13] = (decode_colors[1] + 2*decode_colors[5]) / 3
		decode_colors[14] = (decode_colors[2] + 2*decode_colors[6]) / 3
		decode_colors[15] = 255
	} else {
		//	1 interpolated color, alpha
		decode_colors[8] = (decode_colors[0] + decode_colors[4]) / 2
		decode_colors[9] = (decode_colors[1] + decode_colors[5]) / 2
		decode_colors[10] = (decode_colors[2] + decode_colors[6]) / 2
		decode_colors[11] = 255
		decode_colors[12] = 0
		decode_colors[13] = 0
		decode_colors[14] = 0
		decode_colors[15] = 0
	}

	//	decode the block
	next_bit := uint32(4 * 8)
	for i := 0; i < 16*4; i += 4 {
		idx := ((compressed[next_bit>>3] >> (next_bit & 7)) & 3) * 4
		next_bit += 2
		uncompressed[i+0] = byte(decode_colors[idx+0])
		uncompressed[i+1] = byte(decode_colors[idx+1])
		uncompressed[i+2] = byte(decode_colors[idx+2])
		uncompressed[i+3] = byte(decode_colors[idx+3])
	}
}

func Decode(reader io.Reader) (*Texture, error) {
	var header DDS_Header

	//	load the header
	binary.Read(reader, binary.LittleEndian, &header)

	tex := &Texture{
		Header: header,
		Width:  int(header.Width),
		Height: int(header.Height),
	}

	//	and do some checking
	if string(header.Magic[:]) != "DDS " {
		return tex, fmt.Errorf("invalid header.Magic")
	}

	if header.Size != 124 {
		return tex, fmt.Errorf("invalid header.Size")
	}

	/*
		According to the MSDN spec, the dwFlags should contain
		DDSD_LINEARSIZE if it's compressed, or DDSD_PITCH if
		uncompressed.  Some DDS writers do not conform to the
		spec, so I need to make my reader more tolerant
	*/
	flags := uint32(DDSD_CAPS | DDSD_HEIGHT | DDSD_WIDTH | DDSD_PIXELFORMAT)
	if (header.Flags & flags) != flags {
		return tex, fmt.Errorf("invalid header.Flags")
	}

	if header.PixelFormat.Size != 32 {
		return tex, fmt.Errorf("invalid header.PixelFormat.Size")
	}

	flags = uint32(DDPF_FOURCC | DDPF_RGB)
	//flags = uint32(DDPF_ALPHAPIXELS) (drift/data/grass.dds)
	if (header.PixelFormat.Flags & flags) == 0 {
		return tex, fmt.Errorf("invalid header.PixelFormat.Flags")
	}

	if (header.Caps.Caps1 & DDSCAPS_TEXTURE) == 0 {
		return tex, fmt.Errorf("invalid header.Caps.Caps1")
	}

	// get the image data
	img_x := header.Width
	img_y := header.Height
	img_n := 4
	is_compressed := ((header.PixelFormat.Flags & DDPF_FOURCC) / DDPF_FOURCC) != 0
	has_alpha := ((header.PixelFormat.Flags & DDPF_ALPHAPIXELS) / DDPF_ALPHAPIXELS) != 0
	has_mipmap := ((header.Caps.Caps1 & DDSCAPS_MIPMAP) != 0) && (header.MipMapCount > 1)
	cubemap_faces := ((header.Caps.Caps2 & DDSCAPS2_CUBEMAP) / DDSCAPS2_CUBEMAP)

	// need cubemaps to have square faces
	if img_x == img_y {
		cubemap_faces &= 1
	} else {
		cubemap_faces &= 0
	}
	cubemap_faces *= 5
	cubemap_faces += 1

	block_pitch := (img_x + 3) >> 2
	num_blocks := block_pitch * ((img_y + 3) >> 2)

	/*	let the user know what's going on	*/
	//*x = s->img_x;
	//*y = s->img_y;
	//*comp = s->img_n;

	//fmt.Println(img_x, img_y, img_n, is_compressed, has_alpha, has_mipmap, cubemap_faces, num_blocks, cubemap_faces)

	var dds_data []byte
	var DXT_family uint32

	//	is this uncompressed?
	if is_compressed {
		//	note: header.sPixelFormat.dwFourCC is something like (('D'<<0)|('X'<<8)|('T'<<16)|('1'<<24))
		DXT_family = 1 + (header.PixelFormat.FourCC >> 24) - '1'
		tex.DXT = int(DXT_family)
		if (DXT_family < 1) || (DXT_family > 5) {
			return tex, fmt.Errorf("invalid DXT_family")
		}

		//fmt.Println("DXT_family", DXT_family)

		// check the expected size...oops, nevermind...  those non-compliant writers leave dwPitchOrLinearSize ==

		// passed all the tests, get the RAM for decoding
		block := make([]byte, 16*4)
		compressed := make([]byte, 8)

		sz := img_x * img_y * 4 * cubemap_faces
		dds_data = make([]byte, sz)

		for cf := 0; cf < int(cubemap_faces); cf += 1 {
			//	now read and decode all the blocks
			for i := 0; i < int(num_blocks); i += 1 {
				//	where are we?
				bx := 0
				by := 0
				bw := 4
				bh := 4
				ref_x := 4 * (i % int(block_pitch))
				ref_y := 4 * (i / int(block_pitch))

				// get the next block's worth of compressed data, and decompress it
				if DXT_family == 1 {
					//	DXT1
					io.ReadFull(reader, compressed)
					decode_DXT1_block(block, compressed)
				} else if DXT_family < 4 {
					//	DXT2/3
					io.ReadFull(reader, compressed)
					decode_DXT23_alpha_block(block, compressed)
					io.ReadFull(reader, compressed)
					decode_DXT_color_block(block, compressed)
				} else {
					//	DXT4/5
					io.ReadFull(reader, compressed)
					decode_DXT45_alpha_block(block, compressed)
					io.ReadFull(reader, compressed)
					decode_DXT_color_block(block, compressed)
				}

				//	is this a partial block?
				if ref_x+4 > int(img_x) {
					bw = int(img_x) - ref_x
				}
				if ref_y+4 > int(img_y) {
					bh = int(img_y) - ref_y
				}

				// now drop our decompressed data into the buffer
				for by = 0; by < bh; by += 1 {
					idx := 4 * ((ref_y+by+cf*int(img_x))*int(img_x) + ref_x)
					for bx = 0; bx < bw*4; bx += 1 {
						dds_data[idx+bx] = block[by*16+bx]
					}
				}
			}
			// done reading and decoding the main image...  stbi__skip MIPmaps if present
			if has_mipmap {
				block_size := 16
				if DXT_family == 1 {
					block_size = 8
				}
				for i := 1; i < int(header.MipMapCount); i += 1 {
					mx := int(img_x) >> uint(i+2)
					my := int(img_y) >> uint(i+2)
					if mx < 1 {
						mx = 1
					}
					if my < 1 {
						my = 1
					}
					//stbi__skip( s, mx*my*block_size );
					skipBuf := make([]byte, mx*my*block_size)
					io.ReadFull(reader, skipBuf)
				}
			}
		}
	} else {
		//return 0, 0, nil, fmt.Errorf("TODO uncompressed")
		DXT_family = 0
		img_n = 3
		if has_alpha {
			img_n = 4
		}

		//sz := int(img_x) * int(img_y) * int(img_n) * int(cubemap_faces)
		sz := int(img_x) * int(img_y) * int(img_n) * int(cubemap_faces)
		dds_data = make([]byte, sz)

		// do this once for each face
		for cf := 0; cf < int(cubemap_faces); cf += 1 {

			// TODO: make it efficient.
			//stbi__getn( s, &dds_data[cf*s->img_x*s->img_y*s->img_n], s->img_x*s->img_y*s->img_n );
			faces_buf := make([]byte, img_x*img_y*uint32(img_n))
			io.ReadFull(reader, faces_buf)
			i := uint32(cf) * img_x * img_y * uint32(img_n)
			dds_data = append(dds_data[:i], append(faces_buf, dds_data[i:]...)...)

			// done reading and decoding the main image... stbi__skip MIPmaps if present
			if has_mipmap {
				for i := 1; i < int(header.MipMapCount); i += 1 {
					mx := int(img_x) >> uint(i)
					my := int(img_y) >> uint(i)
					if mx < 1 {
						mx = 1
					}
					if my < 1 {
						my = 1
					}
					skipBuf := make([]byte, mx*my*img_n)
					io.ReadFull(reader, skipBuf)
				}
			}
		}

		var rgba_dds_data []byte
		if img_n == 3 {
			rgba_dds_data = make([]byte, img_x*img_y*4*cubemap_faces)
		}

		// data was BGR, I need it RGB
		for i := 0; i < (sz / img_n); i += 1 {
			offset := i * img_n
			temp := dds_data[offset]
			dds_data[offset] = dds_data[offset+2]
			dds_data[offset+2] = temp

			// always want rgba
			if img_n == 3 {
				alphaOffset := i * 4
				rgba_dds_data[alphaOffset] = dds_data[offset]
				rgba_dds_data[alphaOffset+1] = dds_data[offset+1]
				rgba_dds_data[alphaOffset+2] = dds_data[offset+2]
				rgba_dds_data[alphaOffset+3] = 0xff
			}
		}

		if img_n == 3 {
			dds_data = rgba_dds_data
		}
	}

	tex.Data = dds_data

	//spew.Dump(header)
	return tex, nil
}
