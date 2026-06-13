package shamir

import "testing"

var demoSecrets = [][]byte{
	[]byte("my-super-secret-database-password-12345"), // 39 bytes
	[]byte("my-api-key-xyz123-very-secret"),          // 29 bytes
	[]byte("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ"), // 111 bytes
}

func benchmarkSplitCombine(b *testing.B, k, n int) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		secret := demoSecrets[i%len(demoSecrets)]
		shares, err := Split(secret, n, k)
		if err != nil {
			b.Fatal(err)
		}
		_, err = Combine(shares[:k])
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSplitCombine3of5(b *testing.B)   { benchmarkSplitCombine(b, 3, 5) }
func BenchmarkSplitCombine5of10(b *testing.B)  { benchmarkSplitCombine(b, 5, 10) }
func BenchmarkSplitCombine7of15(b *testing.B)  { benchmarkSplitCombine(b, 7, 15) }

func BenchmarkCombine39Byte3of5(b *testing.B) {
	secret := demoSecrets[0]
	shares, err := Split(secret, 5, 3)
	if err != nil {
		b.Fatal(err)
	}
	use := shares[:3]
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = Combine(use)
		if err != nil {
			b.Fatal(err)
		}
	}
}
