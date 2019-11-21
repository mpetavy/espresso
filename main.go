package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/mpetavy/common"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// Jnlp element
type Jnlp struct {
	XMLName         xml.Name
	Spec            string          `xml:"spec,attr"`
	Codebase        string          `xml:"codebase,attr"`
	Information     Information     `xml:"information"`
	Resources       []Resource      `xml:"resources"`
	PrivateJres     []PrivateJre    `xml:"private_jre"`
	ApplicationDesc ApplicationDesc `xml:"application-desc"`
	AppletDesc      AppletDesc      `xml:"applet-desc"`
}

// Information element
type Information struct {
	XMLName     xml.Name
	Title       string `xml:"title"`
	Vendor      string `xml:"vendor"`
	Homepage    string `xml:"homepage"`
	Description string `xml:"description"`
	Icon        Icon   `xml:"icon"`
}

// Icon element
type Icon struct {
	Href string `xml:"href,attr"`
}

// ApplicationDesc element
type ApplicationDesc struct {
	XMLName   xml.Name
	MainClass string     `xml:"main-class,attr"`
	Arguments []Argument `xml:"argument"`
}

// Argument element
type Argument struct {
	XMLName xml.Name
	Text    string `xml:",chardata"`
}

// AppletDesc element
type AppletDesc struct {
	XMLName   xml.Name
	MainClass string  `xml:"main-class,attr"`
	Params    []Param `xml:"param"`
}

// Param element
type Param struct {
	XMLName xml.Name
	Text    string `xml:",chardata"`
}

// Resource element
type Resource struct {
	XMLName    xml.Name
	J2se       []J2se      `xml:"j2se"`
	Java       []J2se      `xml:"java"`
	Os         string      `xml:"os,attr"`
	Arch       string      `xml:"arch,attr"`
	Jars       []Jar       `xml:"jar"`
	Nativelibs []Jar       `xml:"nativelib"`
	Extensions []Extension `xml:"extension"`
}

// PrivateJre element
type PrivateJre struct {
	XMLName xml.Name
	Os      string `xml:"os,attr"`
	Arch    string `xml:"arch,attr"`
	Href    string `xml:"href,attr"`
	Path    string
	URL     *url.URL
}

// J2se element
type J2se struct {
	XMLName     xml.Name
	Href        string `xml:"href,attr"`
	Version     string `xml:"version,attr"`
	MaxHeapSize string `xml:"max-heap-size,attr"`
}

// Jar element
type Jar struct {
	XMLName xml.Name
	Href    string `xml:"href,attr"`
	Path    string
	URL     *url.URL
}

// Extension element
type Extension struct {
	XMLName xml.Name
	Href    string `xml:"href,attr"`
	Path    string
	URL     *url.URL
}

var (
	address *string
	jrepath *string
	arch    *string
	cache   *string

	operatingsystem string
	jars            string
	nativelibs      string
	maxheapsize     string
	wg              sync.WaitGroup
	mutex           = &sync.Mutex{}
)

func init() {
	common.Init("1.0.6", "2017", "JNLP app launcher as an alternative to Java Webstart", "mpetavy", common.APACHE, false, nil, nil, run, 0)

	usr, _ := user.Current()

	address = flag.String("url", "", "URL to JNLP file")
	jrepath = flag.String("jre", "", "Path to the java executable file")
	arch = flag.String("arch", runtime.GOARCH, "Used architecture")
	cache = flag.String("cache", fmt.Sprintf("%s%c%s", usr.HomeDir, os.PathSeparator, ".espresso"), "Cache path for permanent caching")
}

// download loads a remote resource via http(s) and stores it to the given filename
func download(href string, filename string) error {
	b, err := common.FileExists(filename)
	if err != nil {
		return nil
	}

	var mustDownload = true

	if b {
		client := &http.Client{}

		response, err := client.Head(href)
		if err != nil {
			return err
		}

		// care about the final close of the response body
		defer func() {
			common.WarnError(response.Body.Close())
		}()

		contentLength, _ := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)

		fs, err := common.FileSize(filename)
		if err != nil {
			return err
		}

		mustDownload = fs != contentLength
	}

	if mustDownload {
		common.Debug(fmt.Sprintf("Download %s --> %s", href, filename))

		client := &http.Client{}

		// get a response from the remote source
		response, err := client.Get(href)
		if err != nil {
			return err
		}

		// care about final cleanup of reponse body
		defer func() {
			common.WarnError(response.Body.Close())
		}()

		// create all parent directories for the given filename
		err = os.MkdirAll(filepath.Dir(filename), common.DefaultDirMode)
		if err != nil {
			return err
		}

		err = common.FileStore(filename, response.Body)
		if err != nil {
			return err
		}
	}

	return nil
}

// runUnzip extract all files to the given path from the given filename
func runUnzip(filename string, path string) error {
	r, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}

	// care about closing the ZIP file
	defer func() {
		common.WarnError(r.Close())
	}()

	// loop over the ZIP content
	for _, f := range r.File {
		path := filepath.Join(path, f.Name)

		// create the destination path
		err := os.MkdirAll(filepath.Dir(path), common.DefaultDirMode)
		if err != nil {
			return err
		}

		// open the source file inside the ZIP file
		zipfile, err := f.Open()
		if err != nil {
			return err
		}

		// source is directory?
		if !f.FileInfo().IsDir() {

			// Use os.Create() since Zip don't store file permissions.
			err := common.FileStore(path, zipfile)
			if err != nil {
				return err
			}
		}

		// closes the zipfile file
		common.WarnError(zipfile.Close())
	}

	return nil
}

// runSelfextract explodes the content of the 7zip self extracting executable file
func runSelfextract(filename string) error {
	cmd := exec.Command(filename, "-y", "-o"+filepath.Dir(filename))
	cmd.Stderr = os.Stderr

	err := cmd.Run()

	return err
}

// runResource operates on the a single resource object and cares about download, runUnzip or extraction
func runResource(wg *sync.WaitGroup, url string, path string, doUnzip bool, doExtract bool, channelError *common.ChannelError) {
	defer wg.Done()

	// first do the download ...
	err := download(url, path)
	if err != nil {
		channelError.Add(err)
		return
	}

	doUnzip = doUnzip || strings.HasSuffix(path, ".zip")
	doExtract = doExtract || strings.HasSuffix(path, ".exe")

	// must the resource be unzipped?
	if doUnzip {
		err = runUnzip(path, filepath.Dir(path))
		if err != nil {
			channelError.Add(err)
			return
		}
	}

	// must the resource be extracted?
	if doExtract {
		err := runSelfextract(path)
		if err != nil {
			channelError.Add(err)
			return
		}
	}
}

func runJnlp(address string, doHeader bool, channelError *common.ChannelError) *Jnlp {
	// try to get the JNLP file
	client := &http.Client{}

	response, err := client.Get(address)
	if err != nil {
		channelError.Add(err)
		return nil
	}

	// care about the final close of the response body
	defer func() {
		common.WarnError(response.Body.Close())
	}()

	// load the JNLP file
	content, err := ioutil.ReadAll(response.Body)
	if err != nil {
		channelError.Add(err)
		return nil
	}

	// convert []byte to string
	body := string(content)

	// check for the HTTP status code
	if response.StatusCode != http.StatusOK {
		os.Exit(1)
	}

	// print the JNLP body
	common.Debug(fmt.Sprintf("JNLP body:\n%s", body))

	// parse the JNLP u
	u, err := url.Parse(address)
	if err != nil {
		channelError.Add(err)
		return nil
	}

	// create the file path in the cache directory for the JNLP file
	jnlpPath := filepath.Join(*cache, common.Trim4Path(u.Host))
	// create the app path in the cache directory for the JNLP file
	appPath := filepath.Join(jnlpPath, "app")

	// create empty jnlp object
	jnlp := Jnlp{}

	content, err = common.ToUTF8(bytes.NewReader(content), common.ISO_8859_1)

	// decode ISO8859 encoded JNLP file
	reader := bytes.NewReader(content)
	decoder := xml.NewDecoder(reader)

	// decode the content of the JNLP content
	err = decoder.Decode(&jnlp)
	if err != nil {
		channelError.Add(err)
		return nil
	}

	codebase := jnlp.Codebase

	if codebase == "" {
		codebase = address[:strings.LastIndex(address, "/")]
	}

	// iterate over the JNLP defined resources
	for _, resource := range jnlp.Resources {

		if len(resource.Arch) > 0 {
			resource.Arch = strings.Title(resource.Arch)
		}

		if len(resource.Os) > 0 {
			resource.Os = strings.ToTitle(resource.Os)
		}

		// is the resouce relevant for the current architecture and OS?
		if (len(resource.Arch) == 0 || common.CompareIgnoreCase(resource.Arch, *arch)) && (len(resource.Os) == 0 || common.CompareIgnoreCase(resource.Os, operatingsystem)) {

			// iterate over the resource JARS
			for _, jar := range resource.Jars {

				// inform the WaitGroup that a new resource action will be added
				wg.Add(1)

				// enrich the jar object with destination filepath and URL
				jar.Path = filepath.Join(appPath, jar.Href)
				jar.URL, err = u.Parse(codebase + "/" + jar.Href)
				if err != nil {
					channelError.Add(err)
					return nil
				}

				// append to the jars path list the current resource jar
				mutex.Lock()
				jars = strings.Join([]string{jars, jar.Path}, string(filepath.ListSeparator))
				mutex.Unlock()

				// runResource the resource asynch
				go runResource(&wg, jar.URL.String(), jar.Path, false, false, channelError)
			}

			// iterate over the resource EXTENSIONS
			for _, extension := range resource.Extensions {

				// inform the WaitGroup that a new resource action will be added
				//wg.Add(1)

				// enrich the jar object with destination filepath and URL
				extension.Path = filepath.Join(appPath, extension.Href)
				extension.URL, err = u.Parse(codebase + "/" + extension.Href)
				if err != nil {
					channelError.Add(err)
					return nil
				}

				go runJnlp(extension.URL.String(), false, channelError)
			}

			// iterate over the defined nativelibs
			for _, nativelib := range resource.Nativelibs {

				// inform the WaitGroup that a new resource action will be added
				wg.Add(1)

				// enrich the nativelib object with the destination filepath and URL
				nativelib.Path = filepath.Join(appPath, nativelib.Href)
				nativelib.URL, err = u.Parse(codebase + "/" + nativelib.Href)
				if err != nil {
					channelError.Add(err)
					return nil
				}

				// append to the nativelib path list the current resource nativelib
				mutex.Lock()
				nativelibs = strings.Join([]string{nativelibs, filepath.Dir(nativelib.Path)}, string(filepath.ListSeparator))
				mutex.Unlock()

				// runResource the resource asynch
				go runResource(&wg, nativelib.URL.String(), nativelib.Path, true, false, channelError)
			}

			if doHeader {
				// get the definition of the maxheapsize from the J2SE element
				for _, j2se := range resource.J2se {
					maxheapsize = j2se.MaxHeapSize
				}

				// if no maxheapsize can be found in J2SE element ...
				if len(maxheapsize) == 0 {

					// get the definition of the maxheapsize from the JAVA element
					for _, java := range resource.Java {
						maxheapsize = java.MaxHeapSize
					}
				}
			}
		}
	}

	// iterate over the private JREs
	for _, jre := range jnlp.PrivateJres {

		// is the private JRE relevant for the current architecture and OS?
		if (len(jre.Arch) == 0 || common.CompareIgnoreCase(jre.Arch, *arch)) && (len(jre.Os) == 0 || common.CompareIgnoreCase(jre.Os, operatingsystem)) {

			// inform the WaitGroup that a new resource action will be added
			wg.Add(1)

			var filename string

			// get the filename of the self extracting file
			p := strings.LastIndex(jre.Href, "/")

			if p != -1 {
				filename = jre.Href[p+1:]
			} else {
				filename = jre.Href
			}

			// enrich the JRE object with the destination filepath and URL
			jre.Path = filepath.Join(jnlpPath, jre.Arch, filename)
			jre.URL, err = u.Parse(codebase + "/" + jre.Href)
			if err != nil {
				channelError.Add(err)
				return nil
			}

			if doHeader {
				// get private JRE path
				mutex.Lock()
				*jrepath = filepath.Join(filepath.Dir(jre.Path), "bin", "javaw")
				mutex.Unlock()
			}

			// runResource the resource asynch
			runResource(&wg, jre.URL.String(), jre.Path, false, true, channelError)
		}
	}

	return &jnlp
}

func run() error {
	// if not parameters are provided then show the usage
	if len(os.Args) == 1 {
		flag.Usage()
		os.Exit(1)
	}

	if len(os.Args) == 2 {
		address = &os.Args[1]
	}

	// check if the catch path exists
	b, err := common.FileExists(*cache)
	if err != nil {
		return err
	}

	if !b {
		err := os.MkdirAll(*cache, common.DefaultDirMode)
		if err != nil {
			return err
		}
	}

	// initialize variables due to OS
	switch runtime.GOOS {
	case "linux":
		operatingsystem = strings.ToTitle("Linux")
	case "windows":
		operatingsystem = strings.ToTitle("Windows")
	case "darwin":
		operatingsystem = strings.ToTitle("Mac OS X")
	}

	if len(*jrepath) == 0 {
		// if not private JRE is provided then do the fallback to default JAVAW executable
		if common.IsWindowsOS() {
			*jrepath = "javaw"
		} else {
			*jrepath = "java"
		}
	}

	var channelError common.ChannelError

	jnlp := runJnlp(*address, true, &channelError)

	// wait on all registered WaitGroup objects
	wg.Wait()

	if channelError.Exists() {
		return channelError.Get()
	}

	// cmd line parameters
	var cmds []string

	// if maxheapsize is provided then registr it to the cmds
	if len(maxheapsize) > 0 {
		cmds = append(cmds, "-Xmx"+maxheapsize)
	}

	if nativelibs != "" {
		// add the nativelib objects to the cmds
		cmds = append(cmds, "-Djava.library.path="+nativelibs)
	}

	// add the jars to the cmds
	cmds = append(cmds, "-cp")
	cmds = append(cmds, jars)

	if jnlp.ApplicationDesc.MainClass != "" {
		// add the execution main class to the cmds
		cmds = append(cmds, jnlp.ApplicationDesc.MainClass)

		// add the provided app arguments to the cmds
		for _, argument := range jnlp.ApplicationDesc.Arguments {
			cmds = append(cmds, argument.Text)
		}
	} else {
		// add the execution main class to the cmds
		cmds = append(cmds, jnlp.AppletDesc.MainClass)

		// add the provided app arguments to the cmds
		for _, param := range jnlp.AppletDesc.Params {
			cmds = append(cmds, param.Text)
		}
	}

	common.Debug(fmt.Sprintf("Command line: %s %s", *jrepath, strings.Join(cmds, " ")))

	// initialize the app cmd
	cmd := exec.Command(*jrepath, cmds...)

	// execute the app cmd
	err = cmd.Start()

	return err
}

func main() {
	defer common.Done()

	common.Run([]string{"url"})
}
