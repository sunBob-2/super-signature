# README.md

## 这是什么
一个用go实现的iOS重签名模块，即市面上的iOS超级签名、蒲公英ios内测分发原理

使用本模块可以进行基本的IPA安装包重签名分发

实现功能：苹果开发者账号管理、IPA安装包管理

运行环境：Linux

依赖：openssl, openssl-devel, g++

## 前提（重要）
1.生成ios.csr和ios.key文件
```bash
openssl genrsa -out ios.key 2048
openssl req -new -sha256 -key ios.key -out ios.csr
```
2.需要部署到公网，且需要https（获取UUID过程苹果服务器会回调我们的接口），自行配置ssl证书（项目根目录下的ssl.key、ssl.pem、server.crt和ca.crt）

3.更改app.ini配置文件域名等信息

## 手动部署

```bash
git clone https://github.com/sunBob-2/super-signature.git
# 进入项目zsign目录下
cd super-signature/zsign
# 安装zsign
g++ *.cpp common/*.cpp -lcrypto -std=c++11 -o zsign
sudo cp ./zsign /usr/bin/zsign
# 验证
zsign -h
# 回到项目目录(记得更改app.ini配置信息)
cd ../
# 开启go mod并运行
go env -w GO111MODULE=on
go run main.go
```

## 使用docker部署
```bash
docker-compose up
```

详见`Dockerfile`和`docker-compose.yml`文件

## API接口文档

浏览器访问 https://127.0.0.1:4443/docs/index.html

## 原理
[语雀浏览](https://www.yuque.com/togettoyou/cjqm/rbk50t)

### 基本流程
1. 添加Apple开发者账号(绑定App Store Connect API)
1. 根据描述文件获得用户设备的UDID
1. 借助App Store Connect API在开发者中心添加UDID、创建证书等
1. 重签名（使用zsign开源项目实现在linux服务器上重签名） 
1. 将ipa包上传到服务器上，配置itms-service服务来做分发

> API

![image.png](https://cdn.nlark.com/yuque/0/2021/png/1077776/1614157707280-fc55e268-dc64-4a95-ade2-fb15da135562.png#align=left&display=inline&height=295&margin=%5Bobject%20Object%5D&name=image.png&originHeight=884&originWidth=3294&size=165697&status=done&style=none&width=1098)

`/api/v1/getAllPackage` 返回数据格式
```json
{
  "code": 0,
  "msg": "成功",
  "data": [
    {
      "ID": 1,
      "IconLink": "应用图标地址",
      "BundleIdentifier": "应用包名",
      "Name": "应用名称",
      "Version": "应用版本号",
      "BuildVersion": "应用BuildVersion",
      "MiniVersion": "最低支持ios版本",
      "Summary": "简介",
      "AppLink": "应用下载地址，iPhone使用Safari浏览器访问即可下载",
      "Size": "应用大小",
      "Count": "累计下载量"
    }
  ]
}
```

# 这个程序做了什么？

### 1. 添加Apple开发者账号

> API接口文档：[https://developer.apple.com/documentation/appstoreconnectapi](https://developer.apple.com/documentation/appstoreconnectapi)


使用App Store Connect API需要到[https://appstoreconnect.apple.com/access/api](https://appstoreconnect.apple.com/access/api)生成API密钥P8文件，
以及对应的密钥ID和账号的Issuer ID。
![image.png](https://cdn.nlark.com/yuque/0/2021/png/1077776/1614157937920-e048fc1b-b8ef-4b08-a559-bcf0a9b72c39.png#align=left&display=inline&height=323&margin=%5Bobject%20Object%5D&name=image.png&originHeight=970&originWidth=3284&size=177328&status=done&style=none&width=1094.6666666666667)

正式使用中，使用 API 文档中的 Try it out 按钮来进入测试模式，选择 p8 文件并依次输入 Issuer ID 和密钥 ID，点击 Execute 按钮来提交请求，如无意外开发者账户已储存于数据库中。

token验证关键代码
```go
func (a Authorize) createToken() (string, error) {
	token := &jwt.Token{
		Header: map[string]interface{}{
			"alg": "ES256",
			"kid": a.Kid,
		},
		Claims: jwt.MapClaims{
			"iss": a.Iss,
			"exp": time.Now().Add(time.Second * 60 * 5).Unix(),
			"aud": "appstoreconnect-v1",
		},
		Method: jwt.SigningMethodES256,
	}
	privateKey, err := authKeyFromBytes([]byte(a.P8))
	if err != nil {
		return "", err
	}
	return token.SignedString(privateKey)
}

func authKeyFromBytes(key []byte) (*ecdsa.PrivateKey, error) {
	var err error
	// Parse PEM block
	var block *pem.Block
	if block, _ = pem.Decode(key); block == nil {
		return nil, errors.New("token: AuthKey must be a valid .p8 PEM file")
	}
	// Parse the key
	var parsedKey interface{}
	if parsedKey, err = x509.ParsePKCS8PrivateKey(block.Bytes); err != nil {
		return nil, err
	}
	var pkey *ecdsa.PrivateKey
	var ok bool
	if pkey, ok = parsedKey.(*ecdsa.PrivateKey); !ok {
		return nil, errors.New("token: AuthKey must be of type ecdsa.PrivateKey")
	}
	return pkey, nil
}

```
调用
```go
resp, err := authorize.httpRequest("GET", "https://api.appstoreconnect.apple.com/v1/devices", nil)
defer fasthttp.ReleaseResponse(resp)
if err != nil {
	return err
}
```
这样就可以直接借助App Store Connect API来完成添加udid、创建Certificates证书、创建BundleIds、创建Profile等来实现超级签名的核心功能。


### 2. 获取UDID

#### 创建udid.mobileconfig文件
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
    <dict>
        <key>PayloadContent</key>
        <dict>
            <key>URL</key>
            <string>https://xxx.xxx.org/api/getUDID?id=0</string> //回调接收UDID等信息的，借用这个回调地址将udid传到服务器后台
            <key>DeviceAttributes</key>
            <array>
                <string>UDID</string>
                <string>IMEI</string>
                <string>ICCID</string>
                <string>VERSION</string>
                <string>PRODUCT</string>
            </array>
        </dict>
        <key>PayloadOrganization</key>
        <string>仅用于查询设备UDID安装APP</string>
        <key>PayloadDisplayName</key>
        <string>仅用于查询设备UDID安装APP</string>
        <key>PayloadVersion</key>
        <integer>1</integer>
        <key>PayloadUUID</key>
        <string>c4df5a3a-81e1-430f-b163-d358bc199327</string> //可在https://www.guidgen.com/随机生成
        <key>PayloadIdentifier</key>
        <string>com.togettoyou.UDID-server</string>
        <key>PayloadDescription</key>
        <string>仅用于查询设备UDID安装APP</string>
        <key>PayloadType</key>
        <string>Profile Service</string>
    </dict>
</plist>
```
> iPhone使用Safari浏览器访问放在服务器上的mobileconfig文件，进行安装描述文件，安装完成后苹果会回调我们设置的url，就可以得到udid信息。设置的url是一个post接口，接收到udid信息处理完逻辑后，301重定向到我们需要跳转的网站，如果不301重定向，iPhone会显示安装失败！

![image.png](https://cdn.nlark.com/yuque/0/2021/png/1077776/1615343860515-305320d8-400c-481e-b354-9f334d1db69f.png)
#### 解析苹果返回的Plist信息，提取UDID
```xml
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>IMEI</key>
    <string>12 345678 901234 566789</string>
    <key>PRODUCT</key>
    <string>iPhone10,3</string>
    <key>UDID</key>
    <string>abcd0123456789XXXXXXXXXXXX</string>
    <key>VERSION</key>
    <string>12345</string>
  </dict>
</plist>
```
只需要解析出udid，调用App Store Connect API将UDID添加到苹果开发者中心即可。

### 3. 重签名

> 添加开发者账号之前在本地使用openssl生成后续所需要的key和csr文件

```bash
openssl genrsa -out ios.key 2048
openssl req -new -sha256 -key ios.key -out ios.csr
```
> 利用csr文件调用CreateCertificates (App Store Connect API) 可以生成cer 证书
> 
> 接着利用cer证书生成pem文件（公钥）

```bash
openssl x509 -in ios_development.cer -inform DER -outform PEM -out ios_development.pem
```
> 公钥ios_development.pem、私钥ios.key、描述文件mobileprovision（调用CreateProfile App Store Connect API）、原始ipa
> 四大材料已凑齐！
> 

> 使用开源项目zsign实现重签名(得到新的重签名安装包new.ipa)

```bash
zsign -c ios_development.pem -k ios.key -m 描述文件.mobileprovision  -o new.ipa Runner.ipa
```



### 4. IPA分发

创建后缀为plist的文件，内容如下：
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
        <key>items</key>
        <array>
                <dict>
                        <key>assets</key>
                        <array>
                                <dict>
                                    <key>kind</key>
                                    <string>software-package</string>
                                    <key>url</key>
                                    <string>https://重签名后的ipa下载地址</string>
                                </dict>
                        </array>
                        <key>metadata</key>
                        <dict>
                            <key>bundle-identifier</key>
                            <string>com.togettoyou.app</string>
                            <key>bundle-version</key>
                            <string>1.0.0</string>
                            <key>kind</key>
                            <string>software</string>
                            <key>title</key>
                            <string>App</string>
                        </dict>
                </dict>
        </array>
</dict>
</plist>
```
安装用户需在Safari浏览器访问如下html：
```xml
<a href="itms-services://?action=download-manifest&url={{ .plist下载地址 }}">安装APP</a>
```
![image.png](https://cdn.nlark.com/yuque/0/2021/png/1077776/1615343913212-a39aff60-a561-4d1c-b886-14efdf9eaeed.png#align=left&display=inline&height=575&margin=%5Bobject%20Object%5D&name=image.png&originHeight=1724&originWidth=1034&size=353178&status=done&style=none&width=344.6666666666667)
