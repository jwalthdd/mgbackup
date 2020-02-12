mgbackup
========

Make backups splitting the data in multiples MEGA accounts.

This utility use the package [https://github.com/t3rm1n4l/go-mega](https://github.com/t3rm1n4l/go-mega) to connect with MEGA.
  
### Dependencies
  - go get gopkg.in/yaml.v3
  - go get github.com/t3rm1n4l/go-mega

### Usage
    Set the MEGA accounts up in config/conf.yaml
    
    Usage ./mgbackup:
        Make a backup:         mgbackup -b <folder_local_path>
        Restore a remote dump: mgbackup -r <folder_local_path> <folder_local_destination>

### TODO
  - Use an unlimited number of accounts.
  - List remote dumps available.
  - Download and upload progress bar.
  - Log the activity.
