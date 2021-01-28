# Espresso

## A JNLP app launcher as an alternative to Java Webstart

## Description

Espresso is an alternative to Java Webstart. It uses the same type of implementation technique like Java Webstart but
with better performance. A subset of the official JNLP definition is supported. The improvment is achieved by
multi-threading both the version comparison between server-side and client-side cached components and the download
process of updated or new components. Espresso does not perform an online verification of the digital signatures of the
downloaded JAR files. So Espresso is recommended to be used only in secure intranet environments.

## Private Java Runtime support

Espresso supports the usage of a private Java runtime with the app. This private Java runtime will be also downloaded
and cached with the app in the cache directory. For that Espresso needs either a path to a 32-bit or 64-bit Java runtime
self extracting pacakge in the JNLP definition. If no private Java Runtime is provided with the app then a preinstalled
local Java Runtime is mandatory.

## Sample JNLP file

Here a sample of JNLP with support of Private Java Runtimes.

```
<jnlp spec="1.0+" codebase="http://server">
    <information>
        <title>HelloWorlds</title>
        <vendor>Copyright 2016 JustMe.</vendor>
        <homepage>http://server/about</homepage>
        <description>HelloWorld</description>
        <description kind="short">HelloWorld</description>
        <description kind="tooltip">HelloWorld</description>
        <icon href="helloworld.gif"/>
        <icon href="helloworld.jpg" kind="splash"/>
    </information>
    <security>
        <all-permissions/>
    </security>
    <permissions>
        <all-permissions/>
    </permissions>
    <update check="always" policy="always"/>
    <resources>
        <j2se href="http://java.sun.com/products/autodl/j2se"/>
        <jar href="helloworld.jar"/>
        <extension href="helloworld-extension.jar"/>
    </resources>
    <resources os="Windows" arch="x86">
        <nativelib href="windows_x86/native_windows.jar"/>
    </resources>
    <resources os="Windows" arch="amd64">
        <nativelib href="windows_amd64/native_windows.jar"/>
    </resources>
    <resources os="Linux" arch="x86">
        <nativelib href="linux_x86/native_windows.jar"/>
    </resources>
    <resources os="Linux" arch="amd64">
        <nativelib href="linux_amd64/native_windows.jar"/>
    </resources>
    <private_jre os="Windows" arch="x86" href="private_jre/jre_windows_x86.exe"/>
    <private_jre os="Windows" arch="amd64" href="private_jre/jre_windows_amd64.exe"/>
    <private_jre os="Linux" arch="x86" href="private_jre/jre_linux_x86"/>
    <private_jre os="Linux" arch="amd64" href="private_jre/jre_linux_amd64"/>
    <application-desc main-class="com.justme.HelloWorld"/>
</jnlp>
```

## Usage

```
espresso -url <http(s) url to JNLP application> [-cache <path to the local cache>] [-version] [-v]
```

Parameter | Description
------------ | -------------
-url | Defines to URL to the JNLP application which will be downloaded and executed by Espresso
-cache | Defines to directory of the Espresso cache. The cache stores the latest version of the JNLP components and reuses if needed. If the cache parameter is not defined then the JNLP components are stored in a temporary cache directory ".espresso" in the OS user home directory.
-version | Gives version information about espresso
-v | Verbose information on execution

## Hint and Disclaimer

Use at your own risk.

## License

This software is copyright and protected by the Apache v2.
https://www.apache.org/licenses/LICENSE-2.0.html
