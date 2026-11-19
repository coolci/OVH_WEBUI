# Public catalog ordering fixture

Source: https://eu.api.ovh.com/v1/order/catalog/public/eco?ovhSubsidiary=IE

Captured read-only on 2026-11-07. The fixture retains only planCode, addonFamilies and configurations from 99 plans. No account credentials, customer records or prices are included.

The regression parses all 99 plans into 10,944 hardware/network configuration groups while excluding optional application/distribution licenses. In particular, non-exclusive software license families must never expand into billions of duplicate machine purchases. This is a frozen schema fixture, not an assertion about the current live catalog.
