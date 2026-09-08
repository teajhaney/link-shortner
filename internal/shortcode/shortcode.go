package shortcode

import "strings"

const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"


func EncodeBase62( n uint64) string{
if n == 0{
	return string(base62Alphabet[0])
}

	var sb strings.Builder

	base := uint64(len(base62Alphabet))
	for n > 0{
		remainder := n % base
		sb.WriteString(string(base62Alphabet[remainder]))
		n = n / base
	}
	

	// reverse the string
	encode :=[]byte(sb.String())
	for i,j:=0,len(encode)-1;i<j;i,j=i+1,j-1{
		encode[i],encode[j] = encode[j],encode[i]
	}
	return string(encode)
}
