# Get Script Hash History

This example demonstrates how to retrieve the transaction history for a specific script hash on the BSV blockchain using the Go Wallet Toolbox SDK. It showcases fetching comprehensive transaction data associated with a script hash from blockchain service providers.

## Overview

The process involves several steps:
1. Setting up services configuration with network settings for blockchain data access.
2. Defining the script hash to retrieve transaction history for analysis purposes.
3. Calling `GetScriptHashHistory()` which queries blockchain data services for transaction records.
4. Processing the returned transaction history data including status and block information.
5. Displaying comprehensive transaction records with confirmation status and block heights.

This approach enables comprehensive transaction tracking and analysis for specific script hashes with automatic service redundancy.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable settings:

- **`Script Hash`**: Specific script hash to retrieve transaction history for (default: `"c79e8d823c1ce9b80c9c340a389409f489989800044466c9d05bfef12c472232"`)
- **`Network`**: Blockchain network to query (default: `NetworkMainnet`)
- **`Services Config`**: Default configuration with automatic fallback across multiple blockchain data providers

### Service Setup

The `GetScriptHashHistory` method requires:

- **`Context`**: Request context for lifecycle management
- **`Script Hash`**: Hexadecimal script hash identifier for transaction history retrieval
- **`Services Instance`**: Configured services with fallback logic across WhatsOnChain and other providers

### Response Analysis

The service response contains:

- **`Service Name`**: Which blockchain data service provided the successful transaction history response
- **`Script Hash`**: The queried script hash for verification and confirmation
- **`Transaction History`**: Array of transaction records with hash, status, and block height information
- **`Status Information`**: Confirmation status (Confirmed/Unconfirmed) for each transaction record
- **`Block Heights`**: Block numbers for confirmed transactions, empty for unconfirmed transactions

## Running the Example

To run this example:

```bash
go run ./examples/services_examples/get_script_hash_history/get_script_hash_history.go
```

## Expected Output

```text
🚀 STARTING: Script Hash History
============================================================

=== STEP ===
Wallet-Services is performing: fetching script history for scripthash c79e8d823c1ce9b80c9c340a389409f489989800044466c9d05bfef12c472232
--------------------------------------------------
✅ SUCCESS: Fetched Script Hash History

============================================================
SCRIPT HASH HISTORY
============================================================
Service: WhatsOnChain
ScriptHash: c79e8d823c1ce9b80c9c340a389409f489989800044466c9d05bfef12c472232
Transaction History:
TxHash                                                            Status       Block Height
----------------------------------------------------------------  -----------  ------------
bfe594bc56b11e8f1030e7f6fc53fdc6e58ae05d75838252ab7f7d5f75a09e56  Confirmed    906308
b79f46413372002a2bc01e1c13ab90c896cae993b68cbbab801998f07f834e62  Confirmed    906310
4bf03b27d8fd0e9f7a4f989cbd0834974b2aa00709475246b06d9bd567729f93  Confirmed    906310
4e36756ecf452864560c00b5e60aa8ddf2f1b8a08645cc044b485ab4d234d37a  Confirmed    906311
5d359bc688ff0603cee27c9352dae09dbdb6d3168d0bd52f6a35906ebe59a9b6  Confirmed    906311
744722000a1c2dbc90d32230a0344a093c48fd9a97ac31eb20f93414d7046cb1  Confirmed    906312
17ffb03af472edb959d1bae35aa5972cb410075caeff99743ddc12697ec095ba  Confirmed    906312
13cc5fe2a3c03e9c5f02f926acf02ac413679f170560c419fd89ce338624266f  Confirmed    906313
42b06eb9d338983e09cb69c626790549153a5e4248e04bba24bfab2d09f63d33  Confirmed    906314
22fb65f2fd8006bb3aae87262e1b1782a8851e0e24fd2e7fb5aab545ad409938  Confirmed    906314
9006eee0894b7c58fa5835302f4de443ea5ad950e87be589eff53db8b7ac76c8  Confirmed    906314
ac6cbcbb9147066be423316746b2493a821c52a0b02e196842c5640dbfaa28cf  Confirmed    906315
9f95a5e80736ef4934b0e184b137f72833267c27730f34b03dcaa0465fd0895d  Confirmed    906317
26cf234981876bd934eae21fb2f298333b72777ac77303d77add3dda4ff7ca4d  Confirmed    906318
26a4bd87ec6558af55d82ab1b1490b37b9f7d1a4db1dbf1692031342773b6e17  Confirmed    906320
190dd7f6f311341ddab02d9b963d662d5749d430b583337235d41ea13bd91b2c  Confirmed    906320
c801a7b4d71630cdd9c8a36ed6234cd6b6993b3e4be5e99d73fc84b4f4ab844f  Confirmed    906320
8117023212a9d24008892108e2194c514ca582e367198ee441f3602767e07b15  Confirmed    906321
4baa42160494906561694f864906cb41b0f093f570925b9806656a3bf87aea24  Confirmed    906321
fe12ab2431c7eb5080bc40040885cd913f6e7e1b3e1ef4b43546cc349f169ed1  Confirmed    906322
556c6d31c5e2bc19a1eca07cd8ae6bd82d2aa5d2a9f65eaf8974bbd66566982e  Confirmed    906322
592ea1730de6594a61244c57c27496ad32cc5e8cf0fc5ac47293f554eba9ae58  Confirmed    906322
e541548cebdcaa3b14dfb7e182b5f3e5100740d1558bed76fa813f4c2ac70e53  Confirmed    906322
d10f8887716ec6772e47088871ecc0c2a408753ff90146bc6b6e45160b444f09  Confirmed    906324
58f2751cb54f8d00bbfdd1d90ad199d5b6d1949bed481977e91d6210f1df3221  Confirmed    906324
1396975481f3d763f7936a8f487194e6e472be2d2a86683df0fe8ac460b130db  Confirmed    906324
9a3060f238c557237405a9b7075e7dc47cade7ec3cc9c2203500f256dbe7bdcf  Confirmed    906327
ce60f701ac0c5fc4a56cd3be8331b0168922931a04a1b5d08e4ad612a9a42d23  Confirmed    906328
832acc3476b2205eee453ae65dbfc8cb717102e78a3fdf0471a40a0ff4633e58  Confirmed    906328
89d893e668ed4eb38532bde6f4b1fb052aa6308d327a4cf83b2a38a93b9bc96f  Confirmed    906330
95b1b0e0558e98206aae63e428507f0108e5ea53084cf215e71b14a57d45364d  Confirmed    906331
5402b46c8d3aafe2e16b730583cc7c236c37bf75c9149143660959801b8ae3a3  Confirmed    906331
f0dae4c17253e0645394ef2cc8965663ea67c7f4092a7050b76a7a6cbc03c635  Confirmed    906331
d6b067ed0e58d9f4d3b8355b4e047dc1f7ec50e7a44c82a67fd55b4d4c790a0e  Confirmed    906333
a13b2869b7a8c9afef8f0b70c1a8363d73ac2ef4f38d859cb8c908a2c67f8d74  Confirmed    906333
14f2e86262d09ce7f99ce5561c68fd7779ddd8773dce263e7820ebc3484f13cc  Confirmed    906334
4697cb3b1b4ff82d243253c204a175c01d8c6fc9b3f9ddfe0a0429cb648855e0  Confirmed    906335
ad06c60960e10be79bcb0be7db8e8eb68909c2383f090e35710b262487f17d87  Confirmed    906335
f18f20d37aef97b1287044a29577900a62a5114668453be4c86f51dcf1709234  Confirmed    906336
c79a569644e015e3820667a965d222e8c1378a1e806f684e97d46b29c8089c77  Confirmed    906336
66d49330ebf94eda766947d47e0aa0c62af2b66167554721207205121db6d415  Confirmed    906336
25d2cd0b34300343b769a66f3cb36bc48a4f78c0e65ecf19bf1b7812334500f8  Confirmed    906336
5c47d2dc241fef6c778e98059e636bc9eb0d65e4f6aed3068be3347b9309dbd9  Confirmed    906337
adaeefef4d7544ca2ab412868fe3ab12e581afa9350e663af6e543a1148af214  Confirmed    906337
01ba9fce2d5a96e6a30b71c692ee7af2550d67793526ad8f59ab859936b915bd  Confirmed    906338
796fa4ba8c052e3932f191d13e8889d53f0ea3e83545f3e7432cb68c3eec0200  Confirmed    906338
24ef31304aa88a124aedb61c0aae8f4cdfac1a24e7bb5150135a2036dc2e67e6  Confirmed    906339
1157cc1672bc8a09718e878b7ab5fcff6494eb7a926c2f532e206435fa2643c8  Confirmed    906340
8db096d68b88dc024270780c1f456f874b299968ce5cf43148fb1d2e518da25f  Confirmed    906340
3d9703a03f8f5af0d1b455d63d650c5115cf1230ccbf7845cded619ede786a0d  Confirmed    906340
a4c1ed515efec78da439fbfbe9defc944feb7037db2c105d78fd362f79b95fa4  Confirmed    906340
6b51bcbfa93440b96812e2fcf8f203755f7c29b44d55b4d2cad3f773c8a9b56c  Confirmed    906340
e8389ff35ec8416fe8762d4f6e6f949fe29df83eca32480e79b046371051fb9e  Confirmed    906341
e39876ca510a405a779c2f772f968358b6571f9d59ed745b9c456285ae0aa90b  Confirmed    906341
d75e5e4161bbb3657da39304fea91081531cbaf9c4eb6f91dbbfacd1b62a3f08  Confirmed    906341
6ba6c4edb80895c7cfee5eec0364dd8284efd2f8c7d61902cd1513657615d6fa  Confirmed    906342
ce32e99fc8cbb62da40e3b33e16a85097fb375fdabe6e37aade12b60e9acda93  Confirmed    906342
3cf72d46dc52ee275a0674c3ae0f073865f4b748de78c2439801ef0e507518d5  Confirmed    906342
b048d5ec8c39c929786d1e7d557ebd03c8b8b89c0dee55da4e0d121a41f272a8  Confirmed    906343
d6ff8d32b0b58e52f37496efc3a1ae720182cea87c0c64ef77fed16a27af4504  Confirmed    906345
d38d97da03b59501c91cffc434f9b2e34b12f07493a41fa262fe0bef78c03151  Confirmed    906345
3aca5fce21e88ef7554a7d2c0d79f84361a9429f8772602c228ddd5149f903da  Confirmed    906345
cb296010d711cca5ccd68890e3ae9442f54c25f179884fd8fbabaf2742ae7066  Confirmed    906345
3154c2825e4fc9c941536abc5943050947a66ab824f981ee07c192d70d0231a8  Confirmed    906346
60c9b6541b1b885ce24c3a8d8fdae6302934ef7412dd60851d546a2e33ad769a  Confirmed    906346
7e4f5ae1b1fe8835f5e4027e0caad4756a835e9a9920b8fcdac89b788fd73605  Confirmed    906347
4efd7be530cc3c5c3f9aabb53bee4d14539295c644be791f879b78681e095977  Confirmed    906347
fd9ac54b14346bde8e8d6a6f831410d251c5733c2d94f6edbf561e93f77fdba0  Confirmed    906347
5b04ee1b1695657a2ab8576a56c54c5e132325c9666f55b22c6dfded9a0c2628  Confirmed    906348
23405f3b32610b9f07d2786f1c0298e9eeb48c9812a12ed81d019620371be7f7  Confirmed    906351
425592aaeae72e20a82a5a8fd3b8a42629d34d235760b93c492cf3fe5618029b  Confirmed    906353
0ea44c7ae4c3ed3a8e60b3e224ad0149df51f5c8f9ce38dcfff5fcc21ada44bb  Confirmed    906353
4b4cfc3b084d2c45c3bdaf84bac9624489156e857592fa79affda99ff9ce8a8d  Confirmed    906355
178fae724fd2d6ff7518ec29ce0b1203aa03c10791b36f7aa2d4c120a128946a  Confirmed    906355
ac256dce24e2744df7a19f092a36b22d37d18b59171fc39426a322b44eb5382e  Confirmed    906356
e8e4bad31649405cef7a4887f88d2a3f39e1cd65ae6e3ab109184c87543a3cf7  Confirmed    906357
8f4e3c200a56128251d5655cc182c26665eae265a67ab67900a12b6a096bc8d3  Confirmed    906357
0b67573ffcafeb16e35b6044b412c8b7dfaee7ad3076fab78881d4ded8b47fa9  Confirmed    906357
cf138c287c6ca125bf54c43ce1581dec3ad0e99e5c8cffdcc291d2e5b5faa833  Confirmed    906358
54731007829255720d37fa0e79ee4cbe3bc1b4547967013bce38fd169c86071f  Confirmed    906359
a7ad4ae1a2bd997eeeae604e97b7c828d014c082af8df20d288eecc71a54d3b7  Confirmed    906360
3d81176cb0a88ebd85f41c3ed247adb1806715446b31f7d7a9b6858a86246fd7  Confirmed    906360
c9d6857df2876360ab6ac11743a882b5fe77c0536cb883270365081f1a87ba2b  Confirmed    906361
ccfd940cfd3d5031248cfca9d7bbedcef9904c93df678355dc6a1b52635f78b9  Confirmed    906362
349873b3ace2d81f1f7a185bb9ef3dd2a82ea3436a6a584b4d8d54d1b7489e81  Confirmed    906362
7cbb3f882da11d4497579aa423572dc64bd51aa45c3d92835563b69eda8c3175  Confirmed    906362
870baeba1394ff74dbb81b27109371e128999b9e75d42e74c0f48823bc503efa  Confirmed    906362
3ea135ea012653a94692cd555f20fd72d4ad64657c556f5882f08f7b2aef9fd6  Confirmed    906362
ac9d2862c8669f219f0023a38070bba9d672edb20c0fec912f02617b9c086460  Confirmed    906364
86bd5c8445d72fb2489bd46e29a586c5147ec79d1c9907cfd808c87db8b5f2eb  Confirmed    906365
bdcdb024a9dc87e70a4936e8f963978ff06cbc3a8a719f61457ddc9bef80eb6a  Confirmed    906365
53b137a6184140d68793ed5be45fbe6773c6858a2ee8d3c219343bce3460490a  Confirmed    906365
060df9f02f018be40f9cf8ef4a28c010ab3c74c53d954c9843cc23b1c4b8f94f  Confirmed    906365
2dae3f05444736b9bdc4321c1a1c54647efc83368de0f781279078aefd17493b  Confirmed    906365
f3b06be47e6a965745ee6961234745a8364990a6daf8ac0f38ee2501e26f687e  Confirmed    906367
4bd729ad8e6ade52eb9c8ac7ba9f772079b4e84240894364b7a114d76c652393  Confirmed    906367
dde8a48d9f9ef47f9c82fde31c9b02ec41676902290db938639e99ab6cf5ac7e  Confirmed    906367
7ae8cbe8f1f9f00ffa4524dadad280d2bf7912c86bb0cbcdbce9be5add03be04  Confirmed    906368
db6e02bfe703efabeadd037fc427c2a55b58240f64de0f4a03df459802e5a118  Confirmed    906368
0174964b0042c63622b5e79166a6a62ada539ad40d0ca899bd35f415acf40ede  Confirmed    906369
44256b5a821f83ebf70d578c310a536eb8d51f5d6af68560f36befdd6df0f9e3  Unconfirmed  -
d0a55d2f91f884ae7ff313168003dd197b3cae3db68280c002ad3bd9de75d0ab  Unconfirmed  -
============================================================
🎉 COMPLETED: Script Hash History
```

## Integration Steps

To integrate script hash history retrieval into your application:

1. **Configure services** with appropriate network settings for your target blockchain environment.
2. **Prepare script hash** in hexadecimal format for the script requiring transaction history analysis.
3. **Submit history request** using `GetScriptHashHistory()` with context and script hash parameters.
4. **Process response data** to extract transaction records with status and block height information.
5. **Handle transaction records** by parsing confirmed and unconfirmed transaction data for analysis.
6. **Implement filtering logic** for transaction status, block heights, or specific time periods as needed.
7. **Add monitoring** for script hash activity and new transaction detection across service providers.

## Additional Resources

- [Get Script Hash History Example](./get_script_hash_history.go) - Complete code example for getting script hash transaction history
- [Get Raw Transaction from Transaction ID Documentation](../get_rawtx_from_txid/get_rawtx_from_txid.md) - Get raw transaction data
- [Get Merkle Path for Transaction Documentation](../get_merkle_path_for_tx/get_merkle_path_for_tx.md) - Get cryptographic proof for transactions
