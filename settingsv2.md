## My main goal is to revamp the settings service and make the addition of new settings very straightforward.


### My Apporach


- Service layer:
Functions to address the settings and insertion to the db

- Types:
/settings
    - base.go
        Will contain the base method of validation.
        Convert json -> object based on key
        Convert Object based on key -> JSON
    - invoice_config.go
        - Will have a struct
        - Will have validation business logic


- ServiceLayer Validation:
  - While validating DTO req
    - Validate the value of the setting too

