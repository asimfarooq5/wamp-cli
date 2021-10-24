# Test ticket
wick publish hello --authmethod ticket --authid john --ticket williamsburg

# Test WAMPCRA
wick publish hello --authmethod wampcra --authid john --secret williamsburg

# Test WAMPCRA Salted
wick publish hello --authmethod wampcra --authid wick --secret williamsburg

# Test CryptoSign
wick publish hello --authmethod cryptosign --authid john@wick.com --private-key b99067e6e271ae300f3f5d9809fa09288e96f2bcef8dd54b7aabeb4e579d37ef
