import { ComponentFixture, TestBed } from '@angular/core/testing';

import { DhcpClientsTableComponent } from './dhcp-clients-table.component';

describe('DhcpClientsTableComponent', () => {
  let component: DhcpClientsTableComponent;
  let fixture: ComponentFixture<DhcpClientsTableComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [DhcpClientsTableComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(DhcpClientsTableComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
