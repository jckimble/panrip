# panrip
[![pipeline status](https://gitlab.com/paranoidsecurity/panrip/badges/master/pipeline.svg)](https://gitlab.com/paranoidsecurity/panrip/commits/master)

CLI writen in golang to download music from an pandora account for offline listening

---
* [Install](#install)
* [Recommended Usage](#recommended-usage)
* [License](#license)

---

## Install
```sh
docker pull registry.gitlab.com/paranoidsecurity/panrip
```

## Recommended Usage
This is the quickest way to use panrip. run command and go on with your day.
```sh
docker run -v "/mnt/Music:/download" -d --restart on-failure registry.gitlab.com/paranoidsecurity/panrip --email emailaddress --password password
```

## License
Do What You Want
