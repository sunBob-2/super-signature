package package_service

import (
	"errors"
	"fmt"
	"log"
	"os"
	"super-signature/models"
	"super-signature/pkg/setting"
	"super-signature/pkg/util"
)

type ApplePackage struct {
	ID               int
	IconLink         string
	BundleIdentifier string
	Name             string
	Version          string
	BuildVersion     string
	MiniVersion      string
	Summary          string
	AppLink          string
	Size             float64
	Count            int
}

//获取所有ipa下载地址
func GetAllIPA() ([]ApplePackage, error) {
	var applePackages []ApplePackage
	applePackageList, err := models.GetAllApplePackage()
	if err != nil {
		return nil, err
	}
	for _, v := range applePackageList {
		applePackages = append(applePackages, ApplePackage{
			ID:               v.ID,
			IconLink:         setting.URLSetting.URL + "/api/download?id=" + v.IconLink,
			BundleIdentifier: v.BundleIdentifier,
			Name:             v.Name,
			Version:          v.Version,
			BuildVersion:     v.BuildVersion,
			MiniVersion:      v.MiniVersion,
			Summary:          v.Summary,
			AppLink:          setting.URLSetting.URL + "/api/download?id=" + v.MobileConfigLink,
			Size:             v.Size,
			Count:            v.Count,
		})
	}
	return applePackages, nil
}

//删除指定ipa
func DeleteIPAById(id string) error {
	applePackage, err := models.GetApplePackageByID(id)
	if err != nil {
		return err
	}
	if applePackage == nil {
		return errors.New("IPA包不存在")
	}
	log.Println("------开始删除ipa------")
	err = models.DeleteApplePackageByID(id)
	if err != nil {
		return err
	}
	//删除ipa
	err = util.RunCmd(fmt.Sprintf("rm -rf %s", applePackage.IPAPath))
	if err != nil {
		return err
	}
	//删除icon
	log.Println("删除icon...")
	iconPath, err := models.GetDownloadPathByID(applePackage.IconLink)
	if err != nil {
		return err
	}
	err = os.Remove(iconPath)
	if err != nil {
		return err
	}
	err = models.DeleteDownloadPathByID(applePackage.IconLink)
	if err != nil {
		return err
	}
	//删除描述文件
	log.Println("删除描述文件...")
	mobileConfigPath, err := models.GetDownloadPathByID(applePackage.MobileConfigLink)
	if err != nil {
		return err
	}
	err = os.Remove(mobileConfigPath)
	if err != nil {
		return err
	}
	err = models.DeleteDownloadPathByID(applePackage.MobileConfigLink)
	if err != nil {
		return err
	}
	return nil
}

//解析IPA
func AnalyzeIPA(name, ipaPath, summary string) (*ApplePackage, error) {
	//获取IPA信息
	var ipaPath2 = ipaPath
    ipaPath = ipaPath+".tmp"
    err := util.RunCmd(fmt.Sprintf("mv %s %s",ipaPath2,ipaPath))
	if err != nil {
		log.Printf("%s", err.Error())
		return nil, err
	}
	appInfo, err := util.NewAppParser(setting.PathSetting.UploadPath+name+".png", ipaPath)
	if err != nil {
		return nil, err
	}
	IconPathID, err := models.InsertDownloadPath(appInfo.IconPath)
	if err != nil {
		return nil, err
	}
	//插入到数据库
	applePackage := models.ApplePackage{
		BundleIdentifier: appInfo.Info.CFBundleIdentifier,
		Name:             appInfo.Info.CFBundleName,
		IconLink:         IconPathID,
		Version:          appInfo.Info.CFBundleShortVersion,
		BuildVersion:     appInfo.Info.CFBundleVersion,
		MiniVersion:      appInfo.Info.MinimumOSVersion,
		Summary:          summary,
		MobileConfigLink: "",
		IPAPath:          ipaPath2,
		Size:             util.Decimal(float64(appInfo.Size) / 1000000),
		Count:            0,
	}
	err = applePackage.InsertApplePackage()
	if err != nil {
		return nil, err
	}
	//生成mobileconfig
	mobileconfig, err := creatUDIDMobileconfig(name, applePackage.ID)
	if err != nil {
		return nil, err
	}
	mobileconfigID, err := models.InsertDownloadPath(mobileconfig)
	if err != nil {
		return nil, err
	}
	//更新mobileconfig到数据库
	applePackage.MobileConfigLink = mobileconfigID
	err = applePackage.UpdateApplePackageMobileconfig()
	if err != nil {
		return nil, err
	}
	err = util.RunCmd(fmt.Sprintf("unzip -q %s -d %s/",ipaPath,ipaPath2))
	if err != nil {
		log.Printf("%s", err.Error())
		return nil, err
	}
	err = os.Remove(ipaPath)
	if err != nil {
		return nil, err
	}
	ipaPath = ipaPath2
	err = util.RunCmd(fmt.Sprintf("cp %s/Payload/%s.app/AppIcon60x60@3x.png %s",ipaPath,appInfo.Info.CFBundleExecutable,setting.PathSetting.UploadPath + name + ".png"))
	if err != nil {
		log.Printf("%s", err.Error())
		return nil, err
	}
	return &ApplePackage{
		ID:               applePackage.ID,
		IconLink:         setting.URLSetting.URL + "/api/download?id=" + applePackage.IconLink,
		BundleIdentifier: applePackage.BundleIdentifier,
		Name:             applePackage.Name,
		Version:          applePackage.Version,
		BuildVersion:     applePackage.BuildVersion,
		MiniVersion:      applePackage.MiniVersion,
		Summary:          applePackage.Summary,
		AppLink:          setting.URLSetting.URL + "/api/download?id=" + applePackage.MobileConfigLink,
		Size:             applePackage.Size,
		Count:            applePackage.Count,
	}, nil
}

func creatUDIDMobileconfig(name string, id int) (string, error) {
	var xml = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
    <dict>
        <key>PayloadContent</key>
        <dict>
            <key>URL</key>
            <string>%s/api/addDevice?id=%d</string>
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
        <string>yyyr</string>
        <key>PayloadDisplayName</key>
        <string>设备信息获取</string>
        <key>PayloadVersion</key>
        <integer>1</integer>
        <key>PayloadUUID</key>
        <string>c4df5a3a-81e1-430f-b163-d358bc199327</string>
        <key>PayloadIdentifier</key>
        <string>org.yyyr.signapp</string>
        <key>PayloadDescription</key>
        <string>获取设备信息</string>
        <key>PayloadType</key>
        <string>Profile Service</string>
    </dict>
</plist>`, setting.URLSetting.URL, id)
	var path = setting.PathSetting.UploadPath + name + ".mobileconfig2"
	err := util.CreateFile(xml, path)
	if err != nil {
		return "", err
	}
	var path2 = setting.PathSetting.UploadPath + name + ".mobileconfig"
	err = util.RunCmd(fmt.Sprintf("openssl smime -sign -in %s -out %s -signer ./server.crt -inkey ./ssl.key -certfile ./ca.crt -outform der -nodetach", path, path2))
	if err != nil {
		return "", err
	}
	err = os.Remove(setting.PathSetting.UploadPath + name + ".mobileconfig2")
	if err != nil {
		return "", err
	}
	return path2, nil
}
