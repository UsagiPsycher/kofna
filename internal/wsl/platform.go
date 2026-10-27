// Appels système Windows : registre Lxss, taille allouée, verrous, élévation.
package wsl

import (
	"errors"
	"fmt"
	"kofna/internal/i18n"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const lxssKey = `Software\Microsoft\Windows\CurrentVersion\Lxss`

// regDistro est une entrée de HKCU\...\Lxss\{GUID}.
type regDistro struct {
	ID         string
	Name       string
	Version    int
	BasePath   string
	VhdFile    string
	DefaultUID uint32
}

func registryDistros() ([]regDistro, string, error) {
	root, err := registry.OpenKey(registry.CURRENT_USER, lxssKey, registry.READ)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", i18n.T("err.noRegistry"), err)
	}
	defer root.Close()

	defID, _, _ := root.GetStringValue("DefaultDistribution")
	ids, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return nil, "", err
	}
	var res []regDistro
	for _, id := range ids {
		k, err := registry.OpenKey(root, id, registry.READ)
		if err != nil {
			continue
		}
		d := regDistro{ID: id, Version: 1}
		d.Name, _, _ = k.GetStringValue("DistributionName")
		d.BasePath, _, _ = k.GetStringValue("BasePath")
		d.VhdFile, _, _ = k.GetStringValue("VhdFileName")
		if v, _, err := k.GetIntegerValue("Version"); err == nil {
			d.Version = int(v)
		}
		if v, _, err := k.GetIntegerValue("DefaultUid"); err == nil {
			d.DefaultUID = uint32(v)
		}
		k.Close()
		if d.Name != "" {
			res = append(res, d)
		}
	}
	return res, defID, nil
}

// setRegistryDefaultUID rétablit l'utilisateur par défaut après une réimportation
// (wsl --import repart sur root).
func setRegistryDefaultUID(name string, uid uint32) error {
	regs, _, err := registryDistros()
	if err != nil {
		return err
	}
	for _, d := range regs {
		if strings.EqualFold(d.Name, name) {
			k, err := registry.OpenKey(registry.CURRENT_USER, lxssKey+`\`+d.ID, registry.SET_VALUE)
			if err != nil {
				return err
			}
			defer k.Close()
			return k.SetDWordValue("DefaultUid", uid)
		}
	}
	return errors.New(i18n.T("err.notInRegistry", name))
}

var procGetCompressedFileSizeW = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetCompressedFileSizeW")

// allocatedSize renvoie l'espace réellement occupé sur le disque : pour un fichier
// sparse ou compressé, c'est moins que sa taille logique.
func allocatedSize(path string) (int64, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var high uint32
	low, _, callErr := procGetCompressedFileSizeW.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&high)))
	if uint32(low) == 0xFFFFFFFF && callErr != windows.ERROR_SUCCESS {
		return 0, callErr
	}
	return int64(high)<<32 | int64(uint32(low)), nil
}

func isSparse(path string) bool {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		return false
	}
	return attrs&windows.FILE_ATTRIBUTE_SPARSE_FILE != 0
}

func freeSpace(dir string) (int64, error) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree); err != nil {
		return 0, err
	}
	return int64(free), nil
}

// hideWindow évite qu'une console clignote à chaque appel de wsl.exe.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}

func isAdmin() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

func revealInExplorer(path string) error {
	cmd := exec.Command("explorer.exe", "/select,", path)
	return cmd.Start()
}

// fileLocked indique si un autre processus garde le fichier ouvert : tant que la
// VM WSL n'a pas relâché le VHDX, DiskPart et Optimize-VHD échoueraient.
func fileLocked(path string) bool {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return err == windows.ERROR_SHARING_VIOLATION || err == windows.ERROR_LOCK_VIOLATION
	}
	windows.CloseHandle(h)
	return false
}
