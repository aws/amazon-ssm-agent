// Package commitgraph implements encoding and decoding of commit-graph files.
//
// Git commit graph format
// =======================
//
// The Git commit graph stores a list of commit OIDs and some associated
// metadata, including:
//
// - The generation number of the commit. Commits with no parents have
//   generation number 1; commits with parents have generation number
//   one more than the maximum generation number of its parents. We
//   reserve zero as special, and can be used to mark a generation
//   number invalid or as "not computed".
//
// - The root tree OID.
//
// - The commit date.
//
// - The parents of the commit, stored using positional references within
//   the graph file.
//
// These positional references are stored as unsigned 32-bit integers
// corresponding to the array position within the list of commit OIDs. Due
// to some special constants we use to track parents, we can store at most
// (1 << 30) + (1 << 29) + (1 << 28) - 1 (around 1.8 billion) commits.
//
// == Commit graph files have the following format:
//
// In order to allow extensions that add extra data to the graph, we organize
// the body into "chunks" and provide a binary lookup table at the beginning
// of the body. The header includes certain values, such as number of chunks
// and hash type.
//
// All 4-byte numbers are in network order.
//
// HEADER:
//
//   4-byte signature:
//       The signature is: {'C', 'G', 'P', 'H'}
//
//   1-byte version number:
//       Currently, the only valid version is 1.
//
//   1-byte Hash Version (1 = SHA-1)
//       We infer the hash length (H) from this value.
//
//   1-byte number (C) of "chunks"
//
//   1-byte (reserved for later use)
//      Current clients should ignore this value.
//
// CHUNK LOOKUP:
//
//   (C + 1) * 12 bytes listing the table of contents for the chunks:
//       First 4 bytes describe the chunk id. Value 0 is a terminating label.
//       Other 8 bytes provide the byte-offset in current file for chunk to
//       start. (Chunks are ordered contiguously in the file, so you can infer
//       the length using the next chunk position if necessary.) Each chunk
//       ID appears at most once.
//
//   The remaining data in the body is described one chunk at a time, and
//   these chunks may be given in any order. Chunks are required unless
//   otherwise specified.
//
// CHUNK DATA:
//
//   OID Fanout (ID: {'O', 'I', 'D', 'F'}) (256 * 4 bytes)
//       The ith entry, F[i], stores the number of OIDs with first
//       byte at most i. Thus F[255] stores the total
//       number of commits (N).
//
//   OID Lookup (ID: {'O', 'I', 'D', 'L'}) (N * H bytes)
//       The OIDs for all commits in the graph, sorted in ascending order.
//
//   Commit Data (ID: {'C', 'D', 'A', 'T' }) (N * (H + 16) bytes)
//     * The first H bytes are for the OID of the root tree.
//     * The next 8 bytes are for the positions of the first two parents
//       of the ith commit. Stores value 0x7000000 if no parent in that
//       position. If there are more than two parents, the second value
//       has its most-significant bit on and the other bits store an array
//       position into the Extra Edge List chunk.
//     * The next 8 bytes store the generation number of the commit and
//       the commit time in seconds since EPOCH. The generation number
//       uses the higher 30 bits of the first 4 bytes, while the commit
//       time uses the 32 bits of the second 4 bytes, along with the lowest
//       2 bits of the lowest byte, storing the 33rd and 34th bit of the
//       commit time.
//
//   Extra Edge List (ID: {'E', 'D', 'G', 'E'}) [Optional]
//       This list of 4-byte values store the second through nth parents for
//       all octopus merges. The second parent value in the commit data stores
//       an array position within this list along with the most-significant bit
//       on. Starting at that array position, iterate through this list of commit
//       positions for the parents until reaching a value with the most-significant
//       bit on. The other bits correspond to the position of the last parent.
//
// TRAILER:
//
// 	H-byte HASH-checksum of all of the above.
//
// Source:
// https://raw.githubusercontent.com/git/git/master/Documentation/technical/commit-graph-format.txt
package commitgraph
